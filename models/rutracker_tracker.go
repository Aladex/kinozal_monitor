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
		"login":          {"Вход"},
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
	defer resp.Body.Close()

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
	// Get html of the torrent page
	resp, err := r.user.Client.Get(originalUrl)
	if err != nil {
		r.log.Error("download_torrent", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(r.rutracker1251decoder(resp.Body))
	if err != nil {
		r.log.Error("download_torrent", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get token from html
	token := r.getTokenFromScript(body)
	if token == nil {
		r.log.Error("download_torrent", "Error while getting token from html", map[string]string{"error": "token not found"})
		return nil, fmt.Errorf("token not found")
	}

	// Get topic id from url
	u, err := url.Parse(originalUrl)
	if err != nil {
		r.log.Error("download_torrent", "Error while parsing url", map[string]string{"error": err.Error()})
		return nil, err
	}

	// if query have no id parameter, then return error
	if u.Query().Get("id") == "" {
		r.log.Error("download_torrent", "Error while getting id from url", map[string]string{"error": "id parameter not found"})
		return nil, fmt.Errorf("id parameter not found in url")
	}

	// Get torrent file
	resp, err = r.user.Client.PostForm(fmt.Sprintf("%s/forum/dl.php?t=%s", r.config.BaseURL, u.Query().Get("id")), url.Values{"form_token": {string(token)}})
	if err != nil {
		r.log.Error("download_torrent", "Error while getting torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		r.log.Error("download_torrent", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Check if body is html
	if !CheckBodyIsTorrentFile(body) {
		return nil, fmt.Errorf("body is not torrent file")
	}

	return body, nil
}

// GetTitleFromUrl extracts the title from a rutracker.org torrent page
func (r *RuTrackerTracker) GetTitleFromUrl(url string) (string, error) {
	// For rutracker, we'll extract title from the torrent hash request
	// This is a simplified implementation
	torrent, err := r.GetTorrentHash(url)
	if err != nil {
		return "", err
	}
	return torrent.Name, nil
}

// GetTorrentHash retrieves torrent information including hash from rutracker.org
func (r *RuTrackerTracker) GetTorrentHash(url string) (Torrent, error) {
	// Get html of the torrent page
	resp, err := r.user.Client.Get(url)
	if err != nil {
		r.log.Error("get_torrent_hash", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}
	defer resp.Body.Close()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(r.rutracker1251decoder(resp.Body))
	if err != nil {
		r.log.Error("get_torrent_hash", "Error while reading response body", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	// Get magnet link from html
	magnetLinkRegExp := regexp.MustCompile(`href="magnet:\?xt=urn:btih:([a-z0-9]+)&`)
	magnetLink := magnetLinkRegExp.FindSubmatch(body)
	if len(magnetLink) == 0 {
		r.log.Error("get_torrent_hash", "Error while getting magnet link from html", map[string]string{"error": "magnet link not found"})
		return Torrent{}, fmt.Errorf("magnet link not found")
	}

	return Torrent{Hash: string(magnetLink[1]), Url: url}, nil
}

func (r *RuTrackerTracker) getTokenFromScript(htmlData []byte) []byte {
	fromTokenRegExp := regexp.MustCompile(`form_token: '([a-z0-9]+)',`)
	formToken := fromTokenRegExp.FindSubmatch(htmlData)
	if len(formToken) == 0 {
		return nil
	}
	return formToken[1]
}

func (r *RuTrackerTracker) rutracker1251decoder(reader io.Reader) io.Reader {
	return charmap.Windows1251.NewDecoder().Reader(reader)
}
