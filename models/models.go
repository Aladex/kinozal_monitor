package models

import (
	"net/http"
)

// Torrent is a struct for storing torrent data
type Torrent struct {
	Title string
	Hash  string
	Name  string
	Url   string
}

// TrackerUser is a struct for storing user data
type TrackerUser struct {
	Username string
	Password string
	Client   *http.Client
}
