package models

import (
	"bytes"
	"errors"
	"net/http"
)

// Common errors
var ErrHashIsEmpty = errors.New("hash is empty")

// TorrentTracker interface defines the common methods that all tracker implementations must support
type TorrentTracker interface {
	// Login authenticates the user with the tracker
	Login() error

	// GetTorrentHash retrieves torrent information including hash from a torrent URL
	GetTorrentHash(url string) (Torrent, error)

	// DownloadTorrentFile downloads the torrent file from the tracker
	DownloadTorrentFile(url string) ([]byte, error)

	// GetTitleFromUrl extracts the title from a torrent page
	GetTitleFromUrl(url string) (string, error)

	// DropLoginSession clears the authentication session
	DropLoginSession()

	// GetTrackerName returns the name of the tracker
	GetTrackerName() string
}

// Torrent is a struct for storing torrent data
type Torrent struct {
	Title string
	Hash  string
	Name  string
	Url   string
}

// TrackerUser is a struct for storing user authentication data
type TrackerUser struct {
	Username string
	Password string
	Client   *http.Client
}

// TrackerConfig holds configuration for a specific tracker
type TrackerConfig struct {
	Name      string
	BaseURL   string
	LoginURL  string
	Username  string
	Password  string
	UserAgent string
}

// CheckBodyIsTorrentFile checks if body is torrent file but not html
func CheckBodyIsTorrentFile(body []byte) bool {
	if bytes.Contains(body, []byte("<!DOCTYPE HTML>")) {
		return false
	}
	return true
}
