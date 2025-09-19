package models

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"kinozaltv_monitor/config"
	logger "kinozaltv_monitor/logging"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
)

// KinozalTracker implements the TorrentTracker interface for kinozal.tv
type KinozalTracker struct {
	config TrackerConfig
	user   TrackerUser
	log    *logger.Logger
}

// Export KinozalUser
var KinozalUser = &TrackerUser{
	Username: globalConfig.KinozalUsername,
	Password: globalConfig.KinozalPassword,
}

// NewKinozalTracker creates a new Kinozal tracker instance
func NewKinozalTracker(globalConfig *config.AppConfig) *KinozalTracker {
	return &KinozalTracker{
		config: TrackerConfig{
			Name:      "kinozal",
			BaseURL:   "https://kinozal.tv",
			LoginURL:  "https://kinozal.tv/takelogin.php",
			Username:  globalConfig.KinozalUsername,
			Password:  globalConfig.KinozalPassword,
			UserAgent: globalConfig.UserAgent,
		},
		user: TrackerUser{
			Username: globalConfig.KinozalUsername,
			Password: globalConfig.KinozalPassword,
		},
		log: logger.New("kinozal_tracker"),
	}
}

// GetTrackerName returns the name of the tracker
func (k *KinozalTracker) GetTrackerName() string {
	return k.config.Name
}

// Login authenticates the user with kinozal.tv
func (k *KinozalTracker) Login() error {
	jar, _ := cookiejar.New(nil)
	k.user.Client = &http.Client{
		Jar:     jar,
		Timeout: 100 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if strings.Contains(req.URL.String(), "takelogin.php") {
				return fmt.Errorf("redirect to login page")
			}
			return nil
		},
	}

	data := url.Values{
		"username": {k.user.Username},
		"password": {k.user.Password},
		"returnto": {""},
	}

	req, err := http.NewRequest("POST", k.config.LoginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", k.config.UserAgent)
	req.Header.Set("Referer", k.config.BaseURL)
	req.Header.Set("Origin", k.config.BaseURL)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.user.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(k.kinozal1251decoder(resp.Body))
	if err != nil {
		k.log.Error("login", "Error while reading response body", map[string]string{"error": err.Error()})
		return err
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return err
	}

	var loginError error
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && a.Val == "red" && strings.Contains(n.FirstChild.Data, "Неверно указан пароль для имени") {
					k.log.Info("login", fmt.Sprintf("Wrong password for user %s", k.user.Username), nil)
					loginError = fmt.Errorf("wrong password for user %s on kinozal.tv", k.user.Username)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	if loginError != nil {
		return loginError
	}

	return nil
}

// DropLoginSession clears the authentication session
func (k *KinozalTracker) DropLoginSession() {
	k.user.Client.Jar = nil
}

// DownloadTorrentFile downloads the torrent file from kinozal.tv
func (kz *KinozalTracker) DownloadTorrentFile(url string) ([]byte, error) {
	kz.log.Info("kinozal_download", "Using KinozalTracker.DownloadTorrentFile method", map[string]string{"url": url})

	// Call the TrackerUser's DownloadTorrentFile method which has the actual implementation
	return kz.user.DownloadTorrentFile(url, kz.config.UserAgent)
}

// GetTitleFromUrl extracts the title from a kinozal.tv torrent page
func (k *KinozalTracker) GetTitleFromUrl(originalUrl string) (string, error) {
	req, err := http.NewRequest("GET", originalUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", k.config.UserAgent)

	resp, err := k.user.Client.Do(req)
	if err != nil {
		k.log.Error("get_title_from_url", "Error while sending request", map[string]string{"error": err.Error()})
		return "", err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		k.log.Error("get_title_from_url", "Error while parsing html", map[string]string{"error": err.Error()})
		return "", err
	}

	var title string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			title = n.FirstChild.Data
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	buf := bytes.NewBuffer([]byte(title))
	decoder := charmap.Windows1251.NewDecoder()
	decodedTitle, err := decoder.Bytes(buf.Bytes())
	if err != nil {
		k.log.Error("get_title_from_url", "Error while decoding title", map[string]string{"error": err.Error()})
		return "", err
	}

	return string(decodedTitle), nil
}

// GetTitleFromUrl extracts the title from a kinozal.tv torrent page
func (t *TrackerUser) GetTitleFromUrl(originalUrl string) (string, error) {
	resp, err := t.Client.Get(originalUrl)
	if err != nil {
		log.Error("kinozal_get_title", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("kinozal_get_title", "Error while closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Error("kinozal_get_title", "Error while parsing html", map[string]string{"error": err.Error()})
		return "", err
	}

	var title string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			title = n.FirstChild.Data
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	buf := bytes.NewBuffer([]byte(title))
	decoder := charmap.Windows1251.NewDecoder()
	decodedTitle, err := decoder.Bytes(buf.Bytes())
	if err != nil {
		log.Error("kinozal_get_title", "Error while decoding title", map[string]string{"error": err.Error()})
		return "", err
	}

	return string(decodedTitle), nil
}

// GetTorrentHash retrieves torrent information including hash from kinozal.tv
func (k *KinozalTracker) GetTorrentHash(url string) (Torrent, error) {
	var kzTorrent Torrent

	detailedUrl, err := k.generateUrl(url, "details")
	if err != nil {
		return kzTorrent, err
	}

	for i := 0; i < 10; i++ {
		kzTorrent, err = k.attemptRequest(detailedUrl)
		if err != nil {
			k.handleRequestError(err, url)
		} else if kzTorrent.Hash != "" {
			break
		} else {
			k.user.Client.Jar = nil
			err = k.Login()
			if err != nil {
				k.log.Error("login_err", err.Error(), map[string]string{"url": url})
			}
		}
	}

	if kzTorrent.Hash == "" {
		return kzTorrent, ErrHashIsEmpty
	}

	kzTorrent.Url = url
	return kzTorrent, nil
}

// GetTorrentHash retrieves the torrent hash for KinozalUser
func (t *TrackerUser) GetTorrentHash(url string) (Torrent, error) {
	// Get html of the torrent page
	resp, err := t.Client.Get(url)
	if err != nil {
		log.Error("kinozal_get_torrent_hash", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("kinozal_get_torrent_hash", "Error while closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(rutracker1251decoder(resp.Body))
	if err != nil {
		log.Error("kinozal_get_torrent_hash", "Error while reading response body", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	// Get magnet link from html
	magnetLinkRegExp := regexp.MustCompile(`href="magnet:\?xt=urn:btih:([a-z0-9]+)&`)
	magnetLink := magnetLinkRegExp.FindSubmatch(body)
	if len(magnetLink) == 0 {
		log.Error("kinozal_get_torrent_hash", "Error while getting magnet link from html", map[string]string{"error": "magnet link not found"})
		return Torrent{}, fmt.Errorf("magnet link not found")
	}

	return Torrent{Hash: string(magnetLink[1])}, nil
}

// DownloadTorrentFile downloads the torrent file for KinozalUser
func (t *TrackerUser) DownloadTorrentFile(url string, userAgent string) ([]byte, error) {
	// First, get the details page to extract the torrent download link
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("kinozal_download_torrent", "Error creating request for details page", map[string]string{"error": err.Error()})
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)

	// Use the authenticated client to maintain session cookies
	resp, err := t.Client.Do(req)
	if err != nil {
		log.Error("kinozal_download_torrent", "Error getting details page", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("kinozal_download_torrent", "Unexpected status code for details page", map[string]string{"status": resp.Status})
		return nil, errors.New("unexpected status code for details page: " + resp.Status)
	}

	// Read and decode the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("kinozal_download_torrent", "Error reading details page", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Variable body is not currently used, but we'll keep the extraction of the torrent ID from URL
	// instead of parsing the body content, as it's more reliable
	_ = string(bodyBytes) // Explicitly ignore the body to prevent unused variable warning

	// Extract torrent ID from the URL
	idRegex := regexp.MustCompile(`id=(\d+)`)
	idMatch := idRegex.FindStringSubmatch(url)
	if len(idMatch) < 2 {
		log.Error("kinozal_download_torrent", "Failed to extract torrent ID from URL", map[string]string{"url": url})
		return nil, errors.New("failed to extract torrent ID from URL")
	}

	torrentID := idMatch[1]

	// Construct the actual torrent download URL
	downloadURL := fmt.Sprintf("https://dl.kinozal.tv/download.php?id=%s", torrentID)

	log.Info("kinozal_download_torrent", "Downloading torrent file", map[string]string{
		"details_url":  url,
		"download_url": downloadURL,
		"torrent_id":   torrentID,
	})

	// Now download the actual torrent file
	downloadReq, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		log.Error("kinozal_download_torrent", "Error creating request for torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}

	downloadReq.Header.Set("User-Agent", userAgent)
	downloadReq.Header.Set("Referer", url) // Set referer to the details page

	// Log the request headers for debugging
	headerLog := make(map[string]string)
	for headerName, headerValues := range downloadReq.Header {
		headerLog[headerName] = strings.Join(headerValues, ", ")
	}
	log.Info("kinozal_download_torrent", "Torrent download request headers", headerLog)

	// Use the authenticated client to maintain session cookies
	downloadResp, err := t.Client.Do(downloadReq)
	if err != nil {
		log.Error("kinozal_download_torrent", "Error downloading torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer downloadResp.Body.Close()

	// Log all response headers
	respHeaderLog := make(map[string]string)
	for headerName, headerValues := range downloadResp.Header {
		respHeaderLog[headerName] = strings.Join(headerValues, ", ")
	}
	log.Info("kinozal_download_torrent", "Torrent download response headers", respHeaderLog)

	if downloadResp.StatusCode != http.StatusOK {
		log.Error("kinozal_download_torrent", "Unexpected status code for torrent download", map[string]string{
			"status":       downloadResp.Status,
			"download_url": downloadURL,
		})
		return nil, errors.New("unexpected status code for torrent download: " + downloadResp.Status)
	}

	// Check content type to ensure we're getting a torrent file
	contentType := downloadResp.Header.Get("Content-Type")
	contentDisposition := downloadResp.Header.Get("Content-Disposition")
	log.Info("kinozal_download_torrent", "Response content information", map[string]string{
		"content_type":        contentType,
		"content_disposition": contentDisposition,
		"download_url":        downloadURL,
	})

	if !strings.Contains(contentType, "application/x-bittorrent") &&
		!strings.Contains(contentType, "application/octet-stream") &&
		!strings.Contains(contentDisposition, ".torrent") {
		log.Info("kinozal_download_torrent", "Possibly not a torrent file based on headers", map[string]string{
			"content_type":        contentType,
			"content_disposition": contentDisposition,
			"download_url":        downloadURL,
		})
		// We'll continue anyway but with a warning
	}

	// Read the beginning of the response to check if it looks like a torrent file
	previewBuf := make([]byte, 256)
	n, _ := io.ReadFull(downloadResp.Body, previewBuf)
	if n > 0 {
		// Check if it starts with "d8:announce" which is common for torrent files
		isTorrentLike := false
		if n >= 11 && string(previewBuf[:1]) == "d" {
			isTorrentLike = true
		}

		// Log a preview of the response for debugging
		previewStr := fmt.Sprintf("%q", previewBuf[:n])
		if len(previewStr) > 100 {
			previewStr = previewStr[:100] + "..."
		}
		log.Info("kinozal_download_torrent", "Response body preview", map[string]string{
			"preview":         previewStr,
			"is_torrent_like": fmt.Sprintf("%t", isTorrentLike),
			"preview_bytes":   fmt.Sprintf("%d", n),
		})

		if !isTorrentLike {
			log.Info("kinozal_download_torrent", "Response doesn't look like a torrent file", map[string]string{
				"preview": previewStr,
			})
		}
	}

	// Read the rest of the body
	restBytes, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		log.Error("kinozal_download_torrent", "Error reading torrent file body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Combine the preview and the rest
	torrentFile := append(previewBuf[:n], restBytes...)

	// Log the size of the downloaded torrent file
	log.Info("kinozal_download_torrent", "Successfully downloaded file", map[string]string{
		"size_bytes": fmt.Sprintf("%d", len(torrentFile)),
		"torrent_id": torrentID,
	})

	return torrentFile, nil
}

func (k *KinozalTracker) attemptRequest(url string) (Torrent, error) {
	var kzTorrent Torrent
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return kzTorrent, err
	}

	req.Header.Set("User-Agent", k.config.UserAgent)

	resp, err := k.user.Client.Do(req)
	if err != nil {
		return kzTorrent, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return kzTorrent, err
	}

	kzTorrent = k.parseHtml(doc)
	return kzTorrent, nil
}

func (k *KinozalTracker) handleRequestError(err error, url string) {
	k.log.Error("get_torrent_hash", err.Error(), map[string]string{"url": url})
	k.user.Client.Jar = nil
	err = k.Login()
	if err != nil {
		k.log.Error("login_err", err.Error(), map[string]string{"url": url})
	}
}

func (k *KinozalTracker) parseHtml(doc *html.Node) Torrent {
	var kzTorrent Torrent

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "div" {
					for _, a := range c.Attr {
						if a.Key == "class" && a.Val == "b" {
							kzTorrent.Name = c.FirstChild.Data
						}
					}
				}
				if strings.Contains(c.Data, "Инфо хеш: ") {
					kzTorrent.Hash = strings.TrimPrefix(c.Data, "Инфо хеш: ")
					kzTorrent.Hash = strings.ToLower(strings.TrimSpace(kzTorrent.Hash))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	return kzTorrent
}

func (k *KinozalTracker) kinozal1251decoder(r io.Reader) io.Reader {
	return charmap.Windows1251.NewDecoder().Reader(r)
}

func (k *KinozalTracker) generateUrl(originalURL, linkType string) (string, error) {
	u, err := url.Parse(originalURL)
	if err != nil {
		return "", err
	}

	if u.Query().Get("id") == "" {
		return "", fmt.Errorf("query have no id parameter")
	}

	switch linkType {
	case "details":
		u.Path = "/get_srv_details.php"
		values, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return "", err
		}
		values.Set("action", "2")
		u.RawQuery = values.Encode()
	case "download":
		u.Path = "/download.php"
	}

	return u.String(), nil
}
