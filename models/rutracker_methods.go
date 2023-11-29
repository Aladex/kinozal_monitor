package models

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var RTUser *TrackerUser

var (
	rtBaseURL  = "https://rutracker.org"
	rtLoginURL = "https://rutracker.org/forum/login.php"
)

// RTLogin is a method for logging in to the tracker rutracker.org
func (t *TrackerUser) RTLogin(loginUrl, userAgent, baseURL string) error {
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
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}

func getTokenFromScript(htmlData []byte) []byte {
	fromTokenRegExp := regexp.MustCompile(`form_token: '([a-z0-9]+)',`)
	formToken := fromTokenRegExp.FindSubmatch(htmlData)
	if len(formToken) == 0 {
		return nil
	}
	return formToken[1]
}

func (t *TrackerUser) DownloadRTTorrentFile(originalUrl, userAgent string) ([]byte, error) {
	// Get html of the torrent page
	resp, err := t.Client.Get(originalUrl)
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while getting torrent page", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(kinozal1251decoder(resp.Body))
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get token from html
	token := getTokenFromScript(body)
	if token == nil {
		log.Error("rutracker_download_torrent", "Error while getting token from html", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get topic id from url
	u, err := url.Parse(originalUrl)
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while parsing url", map[string]string{"error": err.Error()})
		return nil, err
	}

	// if query have no id parameter, then return error
	if u.Query().Get("id") == "" {
		log.Error("rutracker_download_torrent", "Error while getting id from url", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Get torrent file
	resp, err = t.Client.PostForm(fmt.Sprintf("%s/forum/dl.php?t=%s", rtBaseURL, u.Query().Get("id")), url.Values{"form_token": {string(token)}})
	if err != nil {
		log.Error("rutracker_download_torrent", "Error while getting torrent file", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(kinozal1251decoder(resp.Body))
	if err != nil {
		log.Error("rutracker_get_torrent_hash", "Error while reading response body", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	// Get magnet link from html
	magnetLinkRegExp := regexp.MustCompile(`href="magnet:\?xt=urn:btih:([a-z0-9]+)&`)
	magnetLink := magnetLinkRegExp.FindSubmatch(body)
	if len(magnetLink) == 0 {
		log.Error("rutracker_get_torrent_hash", "Error while getting magnet link from html", map[string]string{"error": err.Error()})
		return Torrent{}, err
	}

	return Torrent{Hash: string(magnetLink[1])}, nil
}

func init() {
	RTUser = &TrackerUser{
		Username: globalConfig.RtUsername,
		Password: globalConfig.RtPassword,
	}
	err := RTUser.RTLogin(rtLoginURL, userAgent, rtBaseURL)
	if err != nil {
		log.Error("rutracker_init", "Error while logging in", map[string]string{"error": err.Error()})
		// If error is not nil, then exit
		os.Exit(1)
	}
	log.Info("rutracker_init", "Rutracker user logged in", nil)

}
