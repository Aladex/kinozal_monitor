package kinozal

import (
	"golang.org/x/net/html"
	"net/http"
	"strings"
)

// KinozalTorrent is a struct for storing torrent data
type KinozalTorrent struct {
	Title string
	Hash  string
	Name  string
	Url   string
}

// GetTorrentHash is a method for getting torrent hash from kinozal.tv
func (t *TrackerUser) GetTorrentHash(url string) (KinozalTorrent, error) {
	var kzTorrent = KinozalTorrent{
		Url: url,
	}
	// Convert url to detailed url
	detailedUrl, err := generateUrl(url, "details")
	if err != nil {
		return kzTorrent, err
	}
	req, err := http.NewRequest("GET", detailedUrl, nil)
	if err != nil {
		log.Error("kinozal_get_torrent_hash", err.Error(), map[string]string{"url": url})
		return kzTorrent, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		log.Error("kinozal_get_torrent_hash", err.Error(), map[string]string{"url": url})
		return kzTorrent, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Error("kinozal_get_torrent_hash", err.Error(), map[string]string{"url": url})
		return kzTorrent, err
	}

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

	// If hash is empty, then maybe we are not logged in
	// So we need to drop login session and login again
	// And try to get hash again
	if kzTorrent.Hash == "" {
		t.DropLoginSession()
		err = t.Login()
		if err != nil {
			log.Error("kinozal_relogin", err.Error(), map[string]string{"url": url})
			return kzTorrent, err
		}
		return t.GetTorrentHash(url)
	}

	return kzTorrent, nil
}
