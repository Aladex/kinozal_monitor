package qbittorrent

import (
	"bytes"
	"encoding/json"
	"kinozaltv_monitor/config"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
)

var globalConfig = config.GlobalConfig

// QbittorrentUser is a struct for storing user data
type QbittorrentUser struct {
	Username string
	Password string
	Client   *http.Client
}

var GlobalQbittorrentUser *QbittorrentUser

// Torrent is a struct for storing torrent data
type Torrent struct {
	Hash     string `json:"hash"`
	Title    string `json:"title"`
	Name     string `json:"name"`
	Url      string `json:"url"`
	SavePath string `json:"save_path"`
}

// Login is a method for logging in to the tracker
func (qb *QbittorrentUser) Login() error {
	jar, _ := cookiejar.New(nil)
	qb.Client = &http.Client{
		Jar: jar,
	}

	resp, err := qb.Client.PostForm(globalConfig.QBUrl+"/api/v2/auth/login",
		url.Values{"username": {qb.Username}, "password": {qb.Password}})
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// DropLoginSession is a method for dropping the login session by deleting the cookie
func (qb *QbittorrentUser) DropLoginSession() error {
	// Drop login session
	qb.Client.Jar = nil

	return nil
}

// GetTorrentHashList is a method for getting a list of torrent hashes
func (qb *QbittorrentUser) GetTorrentHashList() ([]Torrent, error) {
	// Get torrent list
	resp, err := qb.Client.Get(globalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		return nil, err
	}

	return torrents, nil
}

// AddTorrent is a method for adding a torrent to the client
func (qb *QbittorrentUser) AddTorrent(hash, savePath string, torrent []byte) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("fileselect[]", hash+".torrent")
	if err != nil {
		return err
	}
	_, err = part.Write(torrent)
	if err != nil {
		return err
	}

	// Add the save_path field
	err = writer.WriteField("savepath", savePath)
	if err != nil {
		return err
	}

	// Add other fields
	err = writer.WriteField("autoTMM", "false")
	if err != nil {
		return err
	}
	// add other fields as necessary

	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", globalConfig.QBUrl+"/api/v2/torrents/add", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := qb.Client.Do(req)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// AddTorrentByMagnet is a method for adding a torrent by magnet link
func (qb *QbittorrentUser) AddTorrentByMagnet(hash, downloadPath string) error {
	// Convert hash to magnet
	magnet := "magnet:?xt=urn:btih:" + hash
	// Add torrent by magnet
	resp, err := qb.Client.PostForm(globalConfig.QBUrl+"/api/v2/torrents/add",
		url.Values{"urls": {magnet}, "save_path": {downloadPath}})
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// DeleteTorrent is a method for deleting a torrent by hash
func (qb *QbittorrentUser) DeleteTorrent(hash string, dropFiles bool) error {
	// Convert dropFiles to string
	var dropFilesString string
	if dropFiles {
		dropFilesString = "true"
	} else {
		dropFilesString = "false"
	}

	// POST to api/v2/torrents/delete
	resp, err := qb.Client.PostForm(globalConfig.QBUrl+"/api/v2/torrents/delete",
		url.Values{"hashes": {hash}, "deleteFiles": {dropFilesString}})
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// DeleteTorrentByName is a method for deleting a torrent by name
func (qb *QbittorrentUser) DeleteTorrentByName(torrentName string, dropFiles bool) error {
	// Find torrent by name
	resp, err := qb.Client.Get(globalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		return err
	}

	// Iterate over torrents and find the one with the given name
	var torrentHash string
	for _, torrent := range torrents {
		if torrent.Name == torrentName {
			torrentHash = torrent.Hash
			break
		}
	}

	// Convert dropFiles to string
	var dropFilesString string
	if dropFiles {
		dropFilesString = "true"
	} else {
		dropFilesString = "false"
	}

	// POST to api/v2/torrents/delete with hash as form value
	resp, err = qb.Client.PostForm(globalConfig.QBUrl+"/api/v2/torrents/delete",
		url.Values{"hashes": {torrentHash}, "deleteFiles": {dropFilesString}})

	if err != nil {
		return err
	}

	return nil
}

// GetDownloadPaths is a method for getting a list of download paths from existing torrents
func (qb *QbittorrentUser) GetDownloadPaths() ([]string, error) {
	// Get torrent list
	resp, err := qb.Client.Get(globalConfig.QBUrl + "/api/v2/torrents/info?filter=all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		return nil, err
	}

	// Use a map to count the frequency of each path
	pathCount := make(map[string]int)

	for _, torrent := range torrents {
		pathCount[torrent.SavePath]++
	}

	// Transfer map to a slice of struct to make it sortable
	type pathFreq struct {
		path  string
		count int
	}
	var paths []pathFreq
	for path, count := range pathCount {
		paths = append(paths, pathFreq{path, count})
	}

	// Sort paths slice by frequency
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].count > paths[j].count
	})

	// Create slice of sorted paths
	var sortedPaths []string
	for _, pf := range paths {
		sortedPaths = append(sortedPaths, pf.path)
	}

	return sortedPaths, nil
}

func init() {
	// Initialize user
	GlobalQbittorrentUser = &QbittorrentUser{
		Username: globalConfig.QBUsername,
		Password: globalConfig.QBPassword,
	}

	err := GlobalQbittorrentUser.Login()
	if err != nil {
		panic(err)
	}
}
