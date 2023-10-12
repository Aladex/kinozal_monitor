package common

import (
	"net/url"
)

type TorrentData struct {
	Url          string `json:"url"`
	DownloadPath string `json:"downloadPath"`
}

func GetTrackerDomain(originalUrl string) string {
	u, err := url.Parse(originalUrl)
	if err != nil {
		return ""
	}
	return u.Host
}
