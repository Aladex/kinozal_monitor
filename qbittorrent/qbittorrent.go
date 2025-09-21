package qbittorrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"sync"

	"kinozaltv_monitor/config"
)

// QbittorrentUser is a struct for storing user data
type QbittorrentUser struct {
	Username string
	Password string
	Client   *http.Client
	mutex    sync.Mutex // Add mutex for thread-safe operations
}

// NewQbittorrentUser creates a new QbittorrentUser instance
func NewQbittorrentUser(username, password string) *QbittorrentUser {
	return &QbittorrentUser{
		Username: username,
		Password: password,
	}
}

// Manager holds a qBittorrent user instance
type Manager struct {
	User *QbittorrentUser
}

// NewManager creates a new qBittorrent manager
func NewManager(username, password string) *Manager {
	user := NewQbittorrentUser(username, password)
	return &Manager{
		User: user,
	}
}

// Initialize initializes the qBittorrent connection
func (m *Manager) Initialize() error {
	return m.User.Login()
}

// Global manager instance
var GlobalManager *Manager

// InitializeManager initializes the global qBittorrent manager
func InitializeManager(username, password string) error {
	GlobalManager = NewManager(username, password)
	return GlobalManager.Initialize()
}

// Torrent is a struct for storing torrent data
type Torrent struct {
	Hash     string `json:"hash"`
	Title    string `json:"title"`
	Name     string `json:"name"`
	Url      string `json:"url"`
	SavePath string `json:"save_path"`
}

// isSessionValid checks if the current session is still valid
func (qb *QbittorrentUser) isSessionValid() bool {
	if qb.Client == nil || qb.Client.Jar == nil {
		return false
	}

	// Try to make a simple API call to check session validity
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/app/version")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	// If we get a 403 Forbidden or any non-200 status, session is invalid
	return resp.StatusCode == 200
}

// ensureValidSession ensures we have a valid session, re-authenticating if necessary
func (qb *QbittorrentUser) ensureValidSession() error {
	qb.mutex.Lock()
	defer qb.mutex.Unlock()

	if qb.isSessionValid() {
		return nil
	}

	log.Info("qbittorrent", "Session expired or invalid, re-authenticating", nil)
	return qb.login()
}

// login is the internal login method (without mutex, called from ensureValidSession)
func (qb *QbittorrentUser) login() error {
	jar, _ := cookiejar.New(nil)
	qb.Client = &http.Client{
		Jar: jar,
	}

	log.Info("qbittorrent", "Attempting to login to qbittorrent", map[string]string{"url": config.GlobalConfig.QBUrl + "/api/v2/auth/login", "username": qb.Username})

	resp, err := qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/auth/login",
		url.Values{"username": {qb.Username}, "password": {qb.Password}})
	if err != nil {
		log.Error("qbittorrent", "Error during login request", map[string]string{"error": err.Error()})
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	log.Info("qbittorrent", "Login response received", map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)})

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Login failed with non-200 status", map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)})
		return fmt.Errorf("login failed with status %d", resp.StatusCode)
	}

	// Check if cookies are set
	serverURL, err := url.Parse(config.GlobalConfig.QBUrl)
	if err != nil {
		log.Error("qbittorrent", "Error parsing QB URL for cookie check", map[string]string{"error": err.Error()})
		return err
	}

	cookies := qb.Client.Jar.Cookies(serverURL)
	if len(cookies) == 0 {
		log.Error("qbittorrent", "No cookies received after login", nil)
		return fmt.Errorf("no cookies received after login")
	}

	log.Info("qbittorrent", "Login successful", map[string]string{"cookies_count": fmt.Sprintf("%d", len(cookies))})

	return nil
}

// Login is a method for logging in to the tracker (public method with mutex)
func (qb *QbittorrentUser) Login() error {
	qb.mutex.Lock()
	defer qb.mutex.Unlock()
	return qb.login()
}

// DropLoginSession is a method for dropping the login session by deleting the cookie
func (qb *QbittorrentUser) DropLoginSession() error {
	// Drop login session
	if qb.Client != nil {
		qb.Client.Jar = nil
	}

	return nil
}

// GetTorrentHashList is a method for getting a list of torrent hashes
func (qb *QbittorrentUser) GetTorrentHashList() ([]Torrent, error) {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get torrent list
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		log.Error("qbittorrent", "Failed to get torrent list", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden, attempting to re-authenticate", nil)
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403", map[string]string{"error": err.Error()})
			return nil, err
		}

		// Retry the request
		resp, err = qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
		if err != nil {
			log.Error("qbittorrent", "Failed to get torrent list after re-authentication", map[string]string{"error": err.Error()})
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to get torrent list, non-200 response", map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)})
		return nil, fmt.Errorf("failed to get torrent list, status: %d", resp.StatusCode)
	}

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		log.Error("qbittorrent", "Failed to decode torrent list JSON", map[string]string{"error": err.Error()})
		return nil, err
	}

	return torrents, nil
}

// AddTorrent is a method for adding a torrent to the client
func (qb *QbittorrentUser) AddTorrent(hash, savePath string, torrent []byte) error {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session for adding torrent", map[string]string{"error": err.Error(), "hash": hash})
		return err
	}

	log.Info("qbittorrent", "Adding torrent to qBittorrent", map[string]string{
		"hash":      hash,
		"save_path": savePath,
	})

	// Create a new form data buffer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add torrent file with the correct field name "torrents"
	part, err := writer.CreateFormFile("torrents", hash+".torrent")
	if err != nil {
		log.Error("qbittorrent", "Failed to create form file for torrent", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}
	_, err = part.Write(torrent)
	if err != nil {
		log.Error("qbittorrent", "Failed to write torrent data", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}

	// Add the save_path field
	err = writer.WriteField("savepath", savePath)
	if err != nil {
		log.Error("qbittorrent", "Failed to add save path field", map[string]string{
			"hash":      hash,
			"save_path": savePath,
			"error":     err.Error(),
		})
		return err
	}

	// Add other fields
	err = writer.WriteField("autoTMM", "false")
	if err != nil {
		log.Error("qbittorrent", "Failed to add autoTMM field", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}

	// Close the multipart writer
	err = writer.Close()
	if err != nil {
		log.Error("qbittorrent", "Failed to close multipart writer", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}

	// Create request with the correct content type
	req, err := http.NewRequest("POST", config.GlobalConfig.QBUrl+"/api/v2/torrents/add", body)
	if err != nil {
		log.Error("qbittorrent", "Failed to create request", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Log request details for debugging
	log.Info("qbittorrent", "Sending torrent add request", map[string]string{
		"url":          config.GlobalConfig.QBUrl + "/api/v2/torrents/add",
		"content_type": writer.FormDataContentType(),
		"hash":         hash,
	})

	// Send the request
	resp, err := qb.Client.Do(req)
	if err != nil {
		log.Error("qbittorrent", "Failed to send add torrent request", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while adding torrent, attempting to re-authenticate", map[string]string{"hash": hash})
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while adding torrent", map[string]string{"error": err.Error(), "hash": hash})
			return err
		}

		// Retry the request
		resp, err = qb.Client.Do(req)
		if err != nil {
			log.Error("qbittorrent", "Failed to send add torrent request after re-authentication", map[string]string{
				"hash":  hash,
				"error": err.Error(),
			})
			return err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	// Read response body for better error reporting
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("qbittorrent", "Failed to read response body", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
	}

	// Check response status
	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to add torrent, non-200 response", map[string]string{
			"hash":          hash,
			"status_code":   fmt.Sprintf("%d", resp.StatusCode),
			"response_body": string(responseBody),
		})
		return fmt.Errorf("failed to add torrent, status: %d, response: %s", resp.StatusCode, string(responseBody))
	}

	log.Info("qbittorrent", "Successfully added torrent", map[string]string{
		"hash":      hash,
		"save_path": savePath,
	})

	return nil
}

// AddTorrentByMagnet is a method for adding a torrent by magnet link
func (qb *QbittorrentUser) AddTorrentByMagnet(hash, downloadPath string) error {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session for adding torrent by magnet", map[string]string{"error": err.Error(), "hash": hash})
		return err
	}

	log.Info("qbittorrent", "Adding torrent by magnet link", map[string]string{
		"hash":          hash,
		"download_path": downloadPath,
	})

	// Convert hash to magnet
	magnet := "magnet:?xt=urn:btih:" + hash

	// Add torrent by magnet
	resp, err := qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/torrents/add",
		url.Values{"urls": {magnet}, "save_path": {downloadPath}})
	if err != nil {
		log.Error("qbittorrent", "Failed to add torrent by magnet link", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while adding torrent by magnet, attempting to re-authenticate", map[string]string{"hash": hash})
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while adding torrent by magnet", map[string]string{"error": err.Error(), "hash": hash})
			return err
		}

		// Retry the request
		resp, err = qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/torrents/add",
			url.Values{"urls": {magnet}, "save_path": {downloadPath}})
		if err != nil {
			log.Error("qbittorrent", "Failed to add torrent by magnet link after re-authentication", map[string]string{
				"hash":  hash,
				"error": err.Error(),
			})
			return err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to add torrent by magnet, non-200 response", map[string]string{
			"hash":        hash,
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
		})
		return fmt.Errorf("failed to add torrent by magnet, status: %d", resp.StatusCode)
	}

	log.Info("qbittorrent", "Successfully added torrent by magnet link", map[string]string{
		"hash":          hash,
		"download_path": downloadPath,
	})

	return nil
}

// DeleteTorrent is a method for deleting a torrent by hash
func (qb *QbittorrentUser) DeleteTorrent(hash string, dropFiles bool) error {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session for deleting torrent", map[string]string{"error": err.Error(), "hash": hash})
		return err
	}

	log.Info("qbittorrent", "Deleting torrent by hash", map[string]string{
		"hash":       hash,
		"drop_files": fmt.Sprintf("%t", dropFiles),
	})

	// Convert dropFiles to string
	var dropFilesString string
	if dropFiles {
		dropFilesString = "true"
	} else {
		dropFilesString = "false"
	}

	// POST to api/v2/torrents/delete
	resp, err := qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/torrents/delete",
		url.Values{"hashes": {hash}, "deleteFiles": {dropFilesString}})
	if err != nil {
		log.Error("qbittorrent", "Failed to delete torrent by hash", map[string]string{
			"hash":  hash,
			"error": err.Error(),
		})
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while deleting torrent, attempting to re-authenticate", map[string]string{"hash": hash})
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while deleting torrent", map[string]string{"error": err.Error(), "hash": hash})
			return err
		}

		// Retry the request
		resp, err = qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/torrents/delete",
			url.Values{"hashes": {hash}, "deleteFiles": {dropFilesString}})
		if err != nil {
			log.Error("qbittorrent", "Failed to delete torrent by hash after re-authentication", map[string]string{
				"hash":  hash,
				"error": err.Error(),
			})
			return err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to delete torrent, non-200 response", map[string]string{
			"hash":        hash,
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
		})
		return fmt.Errorf("failed to delete torrent, status: %d", resp.StatusCode)
	}

	log.Info("qbittorrent", "Successfully deleted torrent", map[string]string{
		"hash":       hash,
		"drop_files": fmt.Sprintf("%t", dropFiles),
	})

	return nil
}

// DeleteTorrentByName is a method for deleting a torrent by name
func (qb *QbittorrentUser) DeleteTorrentByName(torrentName string, dropFiles bool) error {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session for deleting torrent by name", map[string]string{"error": err.Error(), "name": torrentName})
		return err
	}

	log.Info("qbittorrent", "Deleting torrent by name", map[string]string{
		"name":       torrentName,
		"drop_files": fmt.Sprintf("%t", dropFiles),
	})

	// Find torrent by name
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		log.Error("qbittorrent", "Failed to get torrent list for deletion by name", map[string]string{
			"name":  torrentName,
			"error": err.Error(),
		})
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while getting torrent list for deletion by name, attempting to re-authenticate", map[string]string{"name": torrentName})
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while getting torrent list for deletion by name", map[string]string{"error": err.Error(), "name": torrentName})
			return err
		}

		// Retry the request
		resp, err = qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
		if err != nil {
			log.Error("qbittorrent", "Failed to get torrent list for deletion by name after re-authentication", map[string]string{
				"name":  torrentName,
				"error": err.Error(),
			})
			return err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to get torrent list for deletion by name, non-200 response", map[string]string{
			"name":        torrentName,
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
		})
		return fmt.Errorf("failed to get torrent list for deletion by name, status: %d", resp.StatusCode)
	}

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		log.Error("qbittorrent", "Failed to decode torrent list JSON", map[string]string{
			"name":  torrentName,
			"error": err.Error(),
		})
		return err
	}

	// Iterate over torrents and find the one with the given name
	var torrentHash string
	for _, torrent := range torrents {
		if torrent.Name == torrentName {
			torrentHash = torrent.Hash
			break
		}
	}

	if torrentHash == "" {
		log.Error("qbittorrent", "Torrent not found for deletion by name", map[string]string{
			"name": torrentName,
		})
		return fmt.Errorf("torrent with name %s not found", torrentName)
	}

	log.Info("qbittorrent", "Found torrent hash for deletion by name", map[string]string{
		"name": torrentName,
		"hash": torrentHash,
	})

	// Convert dropFiles to string
	var dropFilesString string
	if dropFiles {
		dropFilesString = "true"
	} else {
		dropFilesString = "false"
	}

	// POST to api/v2/torrents/delete with hash as form value
	resp, err = qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/torrents/delete",
		url.Values{"hashes": {torrentHash}, "deleteFiles": {dropFilesString}})
	if err != nil {
		log.Error("qbittorrent", "Failed to delete torrent by name", map[string]string{
			"name":  torrentName,
			"hash":  torrentHash,
			"error": err.Error(),
		})
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while deleting torrent by name, attempting to re-authenticate", map[string]string{"name": torrentName, "hash": torrentHash})
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while deleting torrent by name", map[string]string{"error": err.Error(), "name": torrentName, "hash": torrentHash})
			return err
		}

		// Retry the request
		resp, err = qb.Client.PostForm(config.GlobalConfig.QBUrl+"/api/v2/torrents/delete",
			url.Values{"hashes": {torrentHash}, "deleteFiles": {dropFilesString}})
		if err != nil {
			log.Error("qbittorrent", "Failed to delete torrent by name after re-authentication", map[string]string{
				"name":  torrentName,
				"hash":  torrentHash,
				"error": err.Error(),
			})
			return err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to delete torrent by name, non-200 response", map[string]string{
			"name":        torrentName,
			"hash":        torrentHash,
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
		})
		return fmt.Errorf("failed to delete torrent, status: %d", resp.StatusCode)
	}

	log.Info("qbittorrent", "Successfully deleted torrent by name", map[string]string{
		"name":       torrentName,
		"hash":       torrentHash,
		"drop_files": fmt.Sprintf("%t", dropFiles),
	})

	return nil
}

func (qb *QbittorrentUser) GetDownloadPathByHash(torrentHash string) (string, error) {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session for getting download path by hash", map[string]string{"error": err.Error(), "hash": torrentHash})
		return "", err
	}

	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		log.Error("qbittorrent", "Failed to get torrent list for download path by hash", map[string]string{"hash": torrentHash, "error": err.Error()})
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while getting download path by hash, attempting to re-authenticate", map[string]string{"hash": torrentHash})
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while getting download path by hash", map[string]string{"error": err.Error(), "hash": torrentHash})
			return "", err
		}

		// Retry the request
		resp, err = qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
		if err != nil {
			log.Error("qbittorrent", "Failed to get torrent list for download path by hash after re-authentication", map[string]string{
				"hash":  torrentHash,
				"error": err.Error(),
			})
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to get torrent list for download path by hash, non-200 response", map[string]string{
			"hash":        torrentHash,
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
		})
		return "", fmt.Errorf("failed to get torrent list for download path by hash, status: %d", resp.StatusCode)
	}

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		log.Error("qbittorrent", "Failed to decode torrent list JSON for download path by hash", map[string]string{"hash": torrentHash, "error": err.Error()})
		return "", err
	}

	// Iterate over torrents and find the one with the given hash
	var torrentSavePath string

	for _, torrent := range torrents {
		if torrent.Hash == torrentHash {
			torrentSavePath = torrent.SavePath
			break
		}
	}

	return torrentSavePath, nil
}

// GetDownloadPaths is a method for getting a list of download paths from existing torrents
func (qb *QbittorrentUser) GetDownloadPaths() ([]string, error) {
	// Ensure we have a valid session before making the request
	if err := qb.ensureValidSession(); err != nil {
		log.Error("qbittorrent", "Failed to ensure valid session for getting download paths", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get torrent list
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		log.Error("qbittorrent", "Failed to get torrent list for download paths", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a forbidden response (session expired during request)
	if resp.StatusCode == 403 {
		log.Info("qbittorrent", "Received 403 Forbidden while getting download paths, attempting to re-authenticate", nil)
		// Try to re-authenticate and retry the request
		if err := qb.ensureValidSession(); err != nil {
			log.Error("qbittorrent", "Failed to re-authenticate after 403 while getting download paths", map[string]string{"error": err.Error()})
			return nil, err
		}

		// Retry the request
		resp, err = qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
		if err != nil {
			log.Error("qbittorrent", "Failed to get torrent list for download paths after re-authentication", map[string]string{"error": err.Error()})
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Failed to get torrent list for download paths, non-200 response", map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)})
		return nil, fmt.Errorf("failed to get torrent list for download paths, status: %d", resp.StatusCode)
	}

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		log.Error("qbittorrent", "Failed to decode torrent list JSON for download paths", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Use a map to count the frequency of each path
	pathCount := make(map[string]int)

	for _, torrent := range torrents {
		pathCount[torrent.SavePath]++
	}

	// Transfer map to a slice of struct to make it sortable
	type pathFreq struct {
		path  string
		count int
	}
	var paths []pathFreq
	for path, count := range pathCount {
		paths = append(paths, pathFreq{path, count})
	}

	// Sort paths slice by frequency
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].count > paths[j].count
	})

	// Create slice of sorted paths
	var sortedPaths []string
	for _, pf := range paths {
		sortedPaths = append(sortedPaths, pf.path)
	}

	return sortedPaths, nil
}
