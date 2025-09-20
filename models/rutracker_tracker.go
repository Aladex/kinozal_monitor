package models

import (
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

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/charmap"
)

// RuTrackerTracker implements the TorrentTracker interface for rutracker.org
type RuTrackerTracker struct {
	config TrackerConfig
	user   TrackerUser
	log    *logger.Logger
}

// NewRuTrackerTracker creates a new RuTracker tracker instance
func NewRuTrackerTracker(globalConfig *config.AppConfig) *RuTrackerTracker {
	return &RuTrackerTracker{
		config: TrackerConfig{
			Name:      "rutracker",
			BaseURL:   "https://rutracker.org",
			LoginURL:  "https://rutracker.org/forum/login.php",
			Username:  globalConfig.RtUsername,
			Password:  globalConfig.RtPassword,
			UserAgent: globalConfig.UserAgent,
		},
		user: TrackerUser{
			Username: globalConfig.RtUsername,
			Password: globalConfig.RtPassword,
		},
		log: logger.New("rutracker_tracker"),
	}
}

// GetTrackerName returns the name of the tracker
func (r *RuTrackerTracker) GetTrackerName() string {
	return r.config.Name
}

// Login authenticates the user with rutracker.org
func (r *RuTrackerTracker) Login() error {
	jar, _ := cookiejar.New(nil)
	r.user.Client = &http.Client{
		Jar:     jar,
		Timeout: 100 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if strings.Contains(req.URL.String(), "login.php") {
				return fmt.Errorf("redirect to login page")
			}
			return nil
		},
	}

	data := url.Values{
		"login_username": {r.user.Username},
		"login_password": {r.user.Password},
		"login":          {"Login"},
	}

	req, err := http.NewRequest("POST", r.config.LoginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", r.config.UserAgent)

	resp, err := r.user.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.log.Error("login", "Error closing login response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}

// DropLoginSession clears the authentication session
func (r *RuTrackerTracker) DropLoginSession() {
	r.user.Client.Jar = nil
}

// DownloadTorrentFile downloads the torrent file from rutracker.org
func (r *RuTrackerTracker) DownloadTorrentFile(originalUrl string) ([]byte, error) {
	r.log.Info("rutracker_download", "Using RuTrackerTracker.DownloadTorrentFile method", map[string]string{"url": originalUrl})

	var torrentData []byte
	var err error

	for i := 0; i < 10; i++ {
		torrentData, err = r.attemptDownloadTorrentFile(originalUrl)
		if err != nil {
			r.handleRequestError(err, originalUrl)
		} else if len(torrentData) > 0 && CheckBodyIsTorrentFile(torrentData) {
			break
		} else {
			r.log.Info("rutracker_download", "Invalid torrent data received, retrying with fresh session", map[string]string{"url": originalUrl, "attempt": fmt.Sprintf("%d", i+1)})
			r.user.Client.Jar = nil
			err = r.Login()
			if err != nil {
				r.log.Error("login_err", err.Error(), map[string]string{"url": originalUrl})
			}
		}
	}

	if len(torrentData) == 0 || !CheckBodyIsTorrentFile(torrentData) {
		return nil, fmt.Errorf("failed to download valid torrent file after 10 attempts")
	}

	return torrentData, nil
}

func (r *RuTrackerTracker) attemptDownloadTorrentFile(originalUrl string) ([]byte, error) {
	// Get html of the torrent page
	resp, err := r.user.Client.Get(originalUrl)
	if err != nil {
		r.log.Error("download_torrent", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.log.Error("download_torrent", "Error closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Read response body and decode it from windows-1251 to utf-8
	_, err = io.ReadAll(r.rutracker1251decoder(resp.Body))
	if err != nil {
		r.log.Error("download_torrent", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get topic id from url
	u, err := url.Parse(originalUrl)
	if err != nil {
		r.log.Error("download_torrent", "Error while parsing url", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get topic ID from the URL
	topicID := ""
	if u.Query().Get("t") != "" {
		topicID = u.Query().Get("t")
	} else {
		r.log.Error("download_torrent", "Error while getting topic ID from url", map[string]string{"error": "topic ID parameter not found"})
		return nil, fmt.Errorf("topic ID parameter not found in url")
	}

	// Construct the download URL
	downloadURL := fmt.Sprintf("%s/forum/dl.php?t=%s", r.config.BaseURL, topicID)
	r.log.Info("download_torrent", "Downloading torrent file", map[string]string{
		"details_url":  originalUrl,
		"download_url": downloadURL,
		"topic_id":     topicID,
	})

	// Create download request
	downloadReq, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		r.log.Error("download_torrent", "Error creating request for torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}

	downloadReq.Header.Set("User-Agent", r.config.UserAgent)
	downloadReq.Header.Set("Referer", originalUrl)

	// Log request headers
	headerLog := make(map[string]string)
	for headerName, headerValues := range downloadReq.Header {
		headerLog[headerName] = strings.Join(headerValues, ", ")
	}
	r.log.Info("download_torrent", "Request headers", headerLog)

	// Download the torrent file
	downloadResp, err := r.user.Client.Do(downloadReq)
	if err != nil {
		r.log.Error("download_torrent", "Error downloading torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer func() {
		if closeErr := downloadResp.Body.Close(); closeErr != nil {
			r.log.Error("download_torrent", "Error closing download response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Log response headers
	respHeaderLog := make(map[string]string)
	for headerName, headerValues := range downloadResp.Header {
		respHeaderLog[headerName] = strings.Join(headerValues, ", ")
	}
	r.log.Info("download_torrent", "Response headers", respHeaderLog)

	if downloadResp.StatusCode != http.StatusOK {
		r.log.Error("download_torrent", "Unexpected status code", map[string]string{
			"status":       downloadResp.Status,
			"download_url": downloadURL,
		})
		return nil, fmt.Errorf("unexpected status code: %s", downloadResp.Status)
	}

	// Read the torrent file data
	torrentFile, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		r.log.Error("download_torrent", "Error reading torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Check if body is a torrent file
	if !CheckBodyIsTorrentFile(torrentFile) {
		r.log.Error("download_torrent", "Response is not a torrent file", nil)
		return nil, fmt.Errorf("response is not a torrent file")
	}

	r.log.Info("download_torrent", "Successfully downloaded torrent file", map[string]string{
		"size_bytes": fmt.Sprintf("%d", len(torrentFile)),
		"topic_id":   topicID,
	})

	return torrentFile, nil
}

// GetTitleFromUrl extracts the title from a rutracker.org torrent page
func (r *RuTrackerTracker) GetTitleFromUrl(url string) (string, error) {
	var title string
	var err error

	for i := 0; i < 10; i++ {
		title, err = r.attemptGetTitle(url)
		if err != nil {
			r.handleRequestError(err, url)
		} else if title != "" && title != "Unknown RuTracker Torrent" {
			break
		} else {
			r.log.Info("get_title_from_url", "Empty or fallback title received, retrying with fresh session", map[string]string{"url": url, "attempt": fmt.Sprintf("%d", i+1)})
			r.user.Client.Jar = nil
			err = r.Login()
			if err != nil {
				r.log.Error("login_err", err.Error(), map[string]string{"url": url})
			}
		}
	}

	if title == "" {
		return "Unknown RuTracker Torrent", nil
	}

	return title, nil
}

func (r *RuTrackerTracker) attemptGetTitle(url string) (string, error) {
	// Get the HTML of the torrent page
	resp, err := r.user.Client.Get(url)
	if err != nil {
		r.log.Error("get_title_from_url", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.log.Error("get_title_from_url", "Error closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Create a goquery document from the decoded response body
	doc, err := goquery.NewDocumentFromReader(r.rutracker1251decoder(resp.Body))
	if err != nil {
		r.log.Error("get_title_from_url", "Error while parsing HTML document", map[string]string{"error": err.Error()})
		return "", err
	}

	// Find the title in the h1.maintitle element with topic-title anchor
	title := ""
	doc.Find("h1.maintitle a#topic-title").Each(func(i int, s *goquery.Selection) {
		if title == "" { // Only get the first match
			title = strings.TrimSpace(s.Text())
		}
	})

	if title != "" {
		r.log.Info("get_title_from_url", "Successfully extracted title using goquery", map[string]string{"title": title})
		return title, nil
	}

	// Fallback: try to find any h1.maintitle element
	doc.Find("h1.maintitle").Each(func(i int, s *goquery.Selection) {
		if title == "" { // Only get the first match
			title = strings.TrimSpace(s.Text())
		}
	})

	if title != "" {
		r.log.Info("get_title_from_url", "Successfully extracted title from h1.maintitle fallback", map[string]string{"title": title})
		return title, nil
	}

	// If we couldn't find the title, fallback to just getting the topic ID
	topicIDRegExp := regexp.MustCompile(`/forum/viewtopic\.php\?t=(\d+)`)
	topicIDMatch := topicIDRegExp.FindSubmatch([]byte(url))
	if len(topicIDMatch) > 1 {
		fallbackTitle := fmt.Sprintf("RuTracker Topic #%s", topicIDMatch[1])
		r.log.Info("get_title_from_url", "Using fallback title with topic ID", map[string]string{"title": fallbackTitle})
		return fallbackTitle, nil
	}

	r.log.Error("get_title_from_url", "Could not extract title from HTML", nil)
	return "Unknown RuTracker Torrent", nil
}

// GetTorrentHash retrieves torrent information including hash from rutracker.org
func (r *RuTrackerTracker) GetTorrentHash(url string) (Torrent, error) {
	var torrent Torrent
	var err error

	for i := 0; i < 10; i++ {
		torrent, err = r.attemptGetTorrentHash(url)
		if err != nil {
			r.handleRequestError(err, url)
		} else if torrent.Hash != "" {
			break
		} else {
			r.log.Info("get_torrent_hash", "Empty hash received, retrying with fresh session", map[string]string{"url": url, "attempt": fmt.Sprintf("%d", i+1)})
			r.user.Client.Jar = nil
			err = r.Login()
			if err != nil {
				r.log.Error("login_err", err.Error(), map[string]string{"url": url})
			}
		}
	}

	if torrent.Hash == "" {
		return torrent, ErrHashIsEmpty
	}

	torrent.Url = url
	return torrent, nil
}

func (r *RuTrackerTracker) attemptGetTorrentHash(url string) (Torrent, error) {
	// Get html of the torrent page
	resp, err := r.user.Client.Get(url)
	if err != nil {
		r.log.Error("get_torrent_hash", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.log.Error("get_torrent_hash", "Error closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(r.rutracker1251decoder(resp.Body))
	if err != nil {
		r.log.Error("get_torrent_hash", "Error while reading response body", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	// Try to get magnet link from html first
	magnetLinkRegExp := regexp.MustCompile(`href="magnet:\?xt=urn:btih:([a-z0-9]+)&`)
	magnetLink := magnetLinkRegExp.FindSubmatch(body)
	if len(magnetLink) > 0 {
		r.log.Info("get_torrent_hash", "Found hash in magnet link", map[string]string{"hash": string(magnetLink[1])})
		return Torrent{Hash: string(magnetLink[1]), Url: url}, nil
	}

	r.log.Info("get_torrent_hash", "Magnet link not found, trying to download torrent file", map[string]string{"url": url})

	// If magnet link isn't found, try to download the torrent file
	torrentData, err := r.attemptDownloadTorrentFile(url)
	if err != nil {
		r.log.Error("get_torrent_hash", "Error downloading torrent file", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	// Extract hash from the downloaded torrent file
	hash, err := GetInfoHashFromTorrentData(torrentData)
	if err != nil {
		r.log.Error("get_torrent_hash", "Error extracting hash from torrent data", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	r.log.Info("get_torrent_hash", "Successfully extracted hash from torrent file", map[string]string{"hash": hash})
	return Torrent{Hash: hash, Url: url}, nil
}

func (r *RuTrackerTracker) getTokenFromScript(htmlData []byte) []byte {
	fromTokenRegExp := regexp.MustCompile(`form_token: '([a-z0-9]+)',`)
	formToken := fromTokenRegExp.FindSubmatch(htmlData)
	if len(formToken) == 0 {
		return nil
	}
	return formToken[1]
}

func (r *RuTrackerTracker) handleRequestError(err error, url string) {
	r.log.Error("request_error", err.Error(), map[string]string{"url": url})
	r.user.Client.Jar = nil
	err = r.Login()
	if err != nil {
		r.log.Error("login_err", err.Error(), map[string]string{"url": url})
	}
}

func (r *RuTrackerTracker) rutracker1251decoder(reader io.Reader) io.Reader {
	return charmap.Windows1251.NewDecoder().Reader(reader)
}
