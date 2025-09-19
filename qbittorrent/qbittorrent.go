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
	"os"
	"sort"
	"strings"

	"kinozaltv_monitor/config"
)

// QbittorrentUser is a struct for storing user data
type QbittorrentUser struct {
	Username string
	Password string
	Client   *http.Client
}

var GlobalQbittorrentUser *QbittorrentUser

// Torrent is a struct for storing torrent data
type Torrent struct {
	Hash     string `json:"hash"`
	Title    string `json:"title"`
	Name     string `json:"name"`
	Url      string `json:"url"`
	SavePath string `json:"save_path"`
}

// Login is a method for logging in to the tracker
func (qb *QbittorrentUser) Login() error {
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
	defer resp.Body.Close()

	log.Info("qbittorrent", "Login response received", map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)})

	if resp.StatusCode != 200 {
		log.Error("qbittorrent", "Login failed with non-200 status", map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)})
		return fmt.Errorf("login failed with status %d", resp.StatusCode)
	}

	// Check if cookies are set
	cookies := qb.Client.Jar.Cookies(&url.URL{Scheme: "https", Host: strings.TrimPrefix(config.GlobalConfig.QBUrl, "https://")})
	if len(cookies) == 0 {
		log.Error("qbittorrent", "No cookies received after login", nil)
		return fmt.Errorf("no cookies received after login")
	}

	log.Info("qbittorrent", "Login successful", map[string]string{"cookies_count": fmt.Sprintf("%d", len(cookies))})

	return nil
}

// DropLoginSession is a method for dropping the login session by deleting the cookie
func (qb *QbittorrentUser) DropLoginSession() error {
	// Drop login session
	qb.Client.Jar = nil

	return nil
}

// GetTorrentHashList is a method for getting a list of torrent hashes
func (qb *QbittorrentUser) GetTorrentHashList() ([]Torrent, error) {
	// Get torrent list
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		return nil, err
	}

	return torrents, nil
}

// AddTorrent is a method for adding a torrent to the client
func (qb *QbittorrentUser) AddTorrent(hash, savePath string, torrent []byte) error {
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
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
	// Get torrent list
	resp, err := qb.Client.Get(config.GlobalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
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

func init() {
	// Initialize user
	GlobalQbittorrentUser = &QbittorrentUser{
		Username: config.GlobalConfig.QBUsername,
		Password: config.GlobalConfig.QBPassword,
	}

	err := GlobalQbittorrentUser.Login()
	if err != nil {
		log.Error("qbittorrent", "Failed to login to qbittorrent", nil)
		os.Exit(1)
	}
}
