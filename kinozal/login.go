package kinozal

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
	"io"
	"kinozaltv_monitor/config"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

var (
	baseURL   = "https://kinozal.tv"
	loginURL  = "https://kinozal.tv/takelogin.php"
	userAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/113.0"
)

// TrackerUser is a struct for storing user data
type TrackerUser struct {
	Username string
	Password string
	Client   *http.Client
}

// KinozalUser is a global variable for storing user data
var KinozalUser *TrackerUser

// Login is a method for logging in to the tracker
func (t *TrackerUser) Login() error {
	jar, _ := cookiejar.New(nil)
	t.Client = &http.Client{
		Jar: jar,
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

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && a.Val == "red" && strings.Contains(n.FirstChild.Data, "Неверно указан пароль") {
					fmt.Println("Login failed")
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return nil
}

// generateUrl is a function for generating url for kinozal.tv for different purposes
func generateUrl(originalURL, linkType string) (string, error) {
	u, err := url.Parse(originalURL)
	if err != nil {
		return "", err
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

func (t *TrackerUser) DownloadTorrentFile(originalUrl string) ([]byte, error) {
	downloadUrl, err := generateUrl(originalUrl, "download")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (t *TrackerUser) GetTitleFromUrl(originalUrl string) (string, error) {
	req, err := http.NewRequest("GET", originalUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read title of the page
	doc, err := html.Parse(resp.Body)
	if err != nil {
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
		return "", err
	}

	return string(decodedTitle), nil
}

func init() {
	// Initialize user
	KinozalUser = &TrackerUser{
		Username: config.GlobalConfig.KinozalUsername,
		Password: config.GlobalConfig.KinozalPassword,
	}

	err := KinozalUser.Login()
	if err != nil {
		panic(err)
	}
}
