package models

import (
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
	"io"
	"kinozaltv_monitor/config"
	logger "kinozaltv_monitor/logging"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

var log = logger.New("kinozal_package")
var globalConfig = config.GlobalConfig
var KinozalUser *TrackerUser

var (
	baseURL   = "https://kinozal.tv"
	loginURL  = "https://kinozal.tv/takelogin.php"
	userAgent = globalConfig.UserAgent
)

var ErrHashIsEmpty = errors.New("hash is empty")

// KinozalLogin is a method for logging in to the tracker kinozal.tv
func (t *TrackerUser) KinozalLogin(loginURL, userAgent, baseURL string) error {
	jar, _ := cookiejar.New(nil)
	t.Client = &http.Client{
		Jar: jar,
		// Set timeout for http client to 10 seconds
		Timeout: 100 * time.Second,
		// Check redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// If redirect to login page
			if strings.Contains(req.URL.String(), "takelogin.php") {
				// Return error
				return fmt.Errorf("redirect to login page")
			}
			// Else return nil
			return nil
		},
	}

	data := url.Values{
		"username": {t.Username},
		"password": {t.Password},
		"returnto": {""},
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", userAgent)
	// Set referer
	req.Header.Set("Referer", baseURL)
	// Set origin
	req.Header.Set("Origin", baseURL)
	// Set content type
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response body and decode it from windows-1251 to utf-8
	body, err := io.ReadAll(kinozal1251decoder(resp.Body))
	if err != nil {
		log.Error("kinozal_login", "Error while reading response body", map[string]string{"error": err.Error()})
		return err
	}

	// Convert body to Node
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
					log.Info("kinozal_login", fmt.Sprintf("Wrong password for user %s", t.Username), nil)
					loginError = fmt.Errorf("wrong password for user %s on kinozal.tv", t.Username)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// If login error is not nil, then return error
	if loginError != nil {
		return loginError
	}

	return nil
}

// DropLoginSession is a method for dropping login session by deleting cookies
func (t *TrackerUser) DropLoginSession() {
	t.Client.Jar = nil
}

// DownloadTorrentFile is a method for downloading torrent file from kinozal.tv
func (t *TrackerUser) DownloadTorrentFile(originalUrl, userAgent string) ([]byte, error) {
	downloadUrl, err := generateUrl(originalUrl, "download")
	if err != nil {
		log.Error("kinozal_download_torrent_file", "Error while generating download url", map[string]string{"error": err.Error()})
		return nil, err
	}
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		log.Error("kinozal_download_torrent_file", "Error while creating new request", map[string]string{"error": err.Error()})
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		log.Error("kinozal_download_torrent_file", "Error while sending request", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("kinozal_download_torrent_file", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Check if body is html
	if !CheckBodyIsTorrentFile(body) {
		return nil, fmt.Errorf("body is not torrent file")
	}

	return body, nil
}

func (t *TrackerUser) GetTitleFromUrl(originalUrl, userAgent string) (string, error) {
	req, err := http.NewRequest("GET", originalUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		log.Error("kinozal_get_title_from_url", "Error while sending request", map[string]string{"error": err.Error()})
		return "", err
	}
	defer resp.Body.Close()

	// Read title of the page
	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Error("kinozal_get_title_from_url", "Error while parsing html", map[string]string{"error": err.Error()})
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

	// Original title is in 1251 encoding so we need to convert it to utf-8
	buf := bytes.NewBuffer([]byte(title))
	decoder := charmap.Windows1251.NewDecoder()
	decodedTitle, err := decoder.Bytes(buf.Bytes())
	if err != nil {
		log.Error("kinozal_get_title_from_url", "Error while decoding title", map[string]string{"error": err.Error()})
		return "", err
	}

	return string(decodedTitle), nil
}

func kinozal1251decoder(r io.Reader) io.Reader {
	return charmap.Windows1251.NewDecoder().Reader(r)
}

// generateUrl is a function for generating url for kinozal.tv for different purposes
func generateUrl(originalURL, linkType string) (string, error) {
	u, err := url.Parse(originalURL)
	if err != nil {
		return "", err
	}

	// if query have no id parameter, then return error
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

// CheckBodyIsTorrentFile is a function for checking if body is torrent file but not html
func CheckBodyIsTorrentFile(body []byte) bool {
	// Check if body is html
	if bytes.Contains(body, []byte("<!DOCTYPE HTML>")) {
		return false
	}
	return true
}

// GetTorrentHash is a method for getting torrent hash from kinozal.tv
func (t *TrackerUser) GetTorrentHash(url string) (Torrent, error) {
	var kzTorrent Torrent

	// Convert url to detailed url
	detailedUrl, err := generateUrl(url, "details")
	if err != nil {
		return kzTorrent, err
	}

	for i := 0; i < 10; i++ {
		kzTorrent, err = t.attemptRequest(detailedUrl)
		if err != nil {
			// Handle error and maybe relogin
			t.handleRequestError(err, url)
		} else if kzTorrent.Hash != "" {
			// If we get hash successfully, break the loop
			break
		} else {
			// If we get empty hash, relogin
			t.Client.Jar = nil
			err = t.KinozalLogin(loginURL, userAgent, baseURL)
			if err != nil {
				log.Error("kinozal_login_err", err.Error(), map[string]string{"url": url})
			}
		}
	}

	// If hash is still empty after 10 attempts, handle as a special case
	if kzTorrent.Hash == "" {
		// Return custom error with text "hash is empty"
		return kzTorrent, ErrHashIsEmpty
	}

	// Set torrent url
	kzTorrent.Url = url

	return kzTorrent, nil
}

func (t *TrackerUser) attemptRequest(url string) (Torrent, error) {
	var kzTorrent Torrent
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return kzTorrent, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		return kzTorrent, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return kzTorrent, err
	}

	kzTorrent = t.parseHtml(doc)

	return kzTorrent, nil
}

func (t *TrackerUser) handleRequestError(err error, url string) {
	log.Error("kinozal_get_torrent_hash", err.Error(), map[string]string{"url": url})
	t.Client.Jar = nil
	err = t.KinozalLogin(baseURL, userAgent, loginURL)
	if err != nil {
		log.Error("kinozal_login_err", err.Error(), map[string]string{"url": url})
	}
}

func (t *TrackerUser) parseHtml(doc *html.Node) Torrent {
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

func init() {
	// Initialize user
	KinozalUser = &TrackerUser{
		Username: globalConfig.KinozalUsername,
		Password: globalConfig.KinozalPassword,
	}

	err := KinozalUser.KinozalLogin(loginURL, userAgent, baseURL)
	if err != nil {
		log.Error("kinozal_init", "Error while logging in", map[string]string{"error": err.Error()})
		// If error is not nil, then exit
		os.Exit(1)
	}
	log.Info("kinozal_init", "Kinozal user logged in", nil)
}
