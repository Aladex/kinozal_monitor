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

	return kzTorrent, nil
}
