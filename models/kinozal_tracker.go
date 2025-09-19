package models

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
	"io"
	"kinozaltv_monitor/config"
	logger "kinozaltv_monitor/logging"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// KinozalTracker implements the TorrentTracker interface for kinozal.tv
type KinozalTracker struct {
	config TrackerConfig
	user   TrackerUser
	log    *logger.Logger
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
func (k *KinozalTracker) DownloadTorrentFile(originalUrl string) ([]byte, error) {
	downloadUrl, err := k.generateUrl(originalUrl, "download")
	if err != nil {
		k.log.Error("download_torrent_file", "Error while generating download url", map[string]string{"error": err.Error()})
		return nil, err
	}

	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		k.log.Error("download_torrent_file", "Error while creating new request", map[string]string{"error": err.Error()})
		return nil, err
	}
	req.Header.Set("User-Agent", k.config.UserAgent)

	resp, err := k.user.Client.Do(req)
	if err != nil {
		k.log.Error("download_torrent_file", "Error while sending request", map[string]string{"error": err.Error()})
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		k.log.Error("download_torrent_file", "Error while reading response body", map[string]string{"error": err.Error()})
		return nil, err
	}

	return body, nil
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
