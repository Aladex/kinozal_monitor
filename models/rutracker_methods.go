package models

import (
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"io"
	"kinozaltv_monitor/config"
	logger "kinozaltv_monitor/logging"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var RTUser *TrackerUser
var log = logger.New("rutracker_package")
var globalConfig = config.GlobalConfig

var (
	baseURL   = "https://rutracker.org"
	loginURL  = "https://rutracker.org/forum/login.php"
	userAgent = globalConfig.UserAgent
)

// RTLogin is a method for logging in to the tracker rutracker.org
func (t *TrackerUser) RTLogin(loginUrl string) error {
	jar, _ := cookiejar.New(nil)
	t.Client = &http.Client{
		Jar: jar,
		// Set timeout for http client to 10 seconds
		Timeout: 100 * time.Second,
		// Check redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// If redirect to login page
			if strings.Contains(req.URL.String(), "login.php") {
				// Return error
				return fmt.Errorf("redirect to login page")
			}
			// Else return nil
			return nil
		},
	}

	data := url.Values{
		"login_username": {t.Username},
		"login_password": {t.Password},
		"login":          {"Вход"},
	}

	req, err := http.NewRequest("POST", loginUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("rutracker_login", "Error while closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}

func rutracker1251decoder(r io.Reader) io.Reader {
	return charmap.Windows1251.NewDecoder().Reader(r)
}

func getTokenFromScript(htmlData []byte) []byte {
	fromTokenRegExp := regexp.MustCompile(`form_token: '([a-z0-9]+)',`)
	formToken := fromTokenRegExp.FindSubmatch(htmlData)
	if len(formToken) == 0 {
		return nil
	}
	return formToken[1]
}

func (t *TrackerUser) DownloadRTTorrentFile(originalUrl string) ([]byte, error) {
	// Get html of the torrent page
	resp, err := t.Client.Get(originalUrl)
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("rutracker_download_torrent", "Error while closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(rutracker1251decoder(resp.Body))
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get token from html
	token := getTokenFromScript(body)
	if token == nil {
		log.Error("rutracker_download_torrent", "Error while getting token from html", map[string]string{"error": "token not found"})
		return nil, fmt.Errorf("token not found")
	}

	// Get topic id from url
	u, err := url.Parse(originalUrl)
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while parsing url", map[string]string{"error": err.Error()})
		return nil, err
	}

	// if query have no id parameter, then return error
	if u.Query().Get("id") == "" {
		log.Error("rutracker_download_torrent", "Error while getting id from url", map[string]string{"error": "id parameter not found"})
		return nil, fmt.Errorf("id parameter not found in url")
	}

	// Get torrent file
	resp, err = t.Client.PostForm(fmt.Sprintf("%s/forum/dl.php?t=%s", baseURL, u.Query().Get("id")), url.Values{"form_token": {string(token)}})
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while getting torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("rutracker_download_torrent", "Error while closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Read response body
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Check if body is html
	if !CheckBodyIsTorrentFile(body) {
		return nil, fmt.Errorf("body is not torrent file")
	}

	return body, nil
}

func (t *TrackerUser) GetRTTorrentHash(url string) (Torrent, error) {
	// Get html of the torrent page
	resp, err := t.Client.Get(url)
	if err != nil {
		log.Error("rutracker_get_torrent_hash", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("rutracker_get_torrent_hash", "Error while closing response body", map[string]string{"error": closeErr.Error()})
		}
	}()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(rutracker1251decoder(resp.Body))
	if err != nil {
		log.Error("rutracker_get_torrent_hash", "Error while reading response body", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	// Get magnet link from html
	magnetLinkRegExp := regexp.MustCompile(`href="magnet:\?xt=urn:btih:([a-z0-9]+)&`)
	magnetLink := magnetLinkRegExp.FindSubmatch(body)
	if len(magnetLink) == 0 {
		log.Error("rutracker_get_torrent_hash", "Error while getting magnet link from html", map[string]string{"error": "magnet link not found"})
		return Torrent{}, fmt.Errorf("magnet link not found")
	}

	return Torrent{Hash: string(magnetLink[1])}, nil
}

func init() {
	RTUser = &TrackerUser{
		Username: globalConfig.RtUsername,
		Password: globalConfig.RtPassword,
	}
	err := RTUser.RTLogin(loginURL)
	if err != nil {
		log.Error("rutracker_init", "Error while logging in", map[string]string{"error": err.Error()})
		// If error is not nil, then exit
		os.Exit(1)
	}
	log.Info("rutracker_init", "Rutracker user logged in", nil)
}
