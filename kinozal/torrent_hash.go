package kinozal

import (
	"errors"
	"golang.org/x/net/html"
	"kinozaltv_monitor/models"
	"net/http"
	"strings"
)

var ErrHashIsEmpty = errors.New("hash is empty")

// GetTorrentHash is a method for getting torrent hash from kinozal.tv
func (t *TrackerUser) GetTorrentHash(url string) (models.Torrent, error) {
	var kzTorrent models.Torrent

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
			err = t.Login()
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

func (t *TrackerUser) attemptRequest(url string) (models.Torrent, error) {
	var kzTorrent models.Torrent
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
	err = t.Login()
	if err != nil {
		log.Error("kinozal_login_err", err.Error(), map[string]string{"url": url})
	}
}

func (t *TrackerUser) parseHtml(doc *html.Node) models.Torrent {
	var kzTorrent models.Torrent

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
