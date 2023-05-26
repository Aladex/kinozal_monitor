package api

import (
	"github.com/labstack/echo/v4"
	"kinozaltv_monitor/config"
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/kinozal"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/qbittorrent"
	"kinozaltv_monitor/telegram"
)

var kzUser = kinozal.KinozalUser
var qbUser = qbittorrent.GlobalQbittorrentUser
var globalConfig = config.GlobalConfig
var log = logger.New("api")

// AddTorrentUrl is a function for adding a torrent by url
func AddTorrentUrl(c echo.Context) error {
	// Get url from request body by POST method
	url := c.FormValue("url")
	// Get hash from kinozal.tv
	torrentInfo, err := kzUser.GetTorrentHash(url)
	if err != nil {
		// Return 500 Internal Server Error
		return c.String(500, err.Error())
	}
	// Check if torrent exists in qbittorrent
	torrentHashList, err := qbUser.GetTorrentHashList()
	if err != nil {
		// Return 500 Internal Server Error with json error: {"error": "error message"}
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	// Get title from original url
	title, err := kzUser.GetTitleFromUrl(url)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	// Set title to torrentInfo
	torrentInfo.Title = title

	// Add torrent to database
	err = database.CreateOrUpdateRecord(database.DB, torrentInfo)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	for _, hash := range torrentHashList {
		if hash.Hash == torrentInfo.Hash {
			// Return 409 Conflict
			return c.JSON(200, map[string]string{"status": "ok"})
		}
	}
	// Get torrent file from kinozal.tv
	torrentFile, err := kzUser.DownloadTorrentFile(url)
	if err != nil {
		log.Info("download_torrent_file", err.Error(), map[string]string{
			"torrent_url": url,
			"reason":      "torrent file not found",
			"result":      "try to add by magnet link",
		})
		// Add torrent by magnet
		err = qbUser.AddTorrentByMagnet(torrentInfo.Hash)
		if err != nil {
			// Return 500 Internal Server Error
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
	} else {
		// Add torrent to qbittorrent
		err = qbUser.AddTorrent(torrentInfo.Hash, torrentFile)
		if err != nil {
			// Return 500 Internal Server Error
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
	}

	// Send telegram message about adding torrent in goroutine
	go func() {
		err = telegram.SendTorrentAction("added", globalConfig.TelegramToken, torrentInfo)
		if err != nil {
			log.Error("send_telegram_message", err.Error(), nil)
		}
	}()

	return c.JSON(200, map[string]string{"status": "ok"})
}

// RemoveTorrentUrl is a function for removing a torrent by ID
func RemoveTorrentUrl(c echo.Context) error {
	// Read JSON from request body
	var json map[string]string
	err := c.Bind(&json)
	if err != nil {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	// Get torrent name from JSON
	torrentUrl := json["url"]
	torrentHash := json["hash"]
	// Delete torrent from qbittorrent by name
	err = qbUser.DeleteTorrent(torrentHash, true)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	// Delete torrent from database
	err = database.DeleteRecord(database.DB, torrentUrl)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	return c.JSON(200, map[string]string{"status": "ok"})
}

// GetTorrentList is a function for getting a list of torrents
func GetTorrentList(c echo.Context) error {
	dbTorrents, err := database.GetAllRecords(database.DB)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	// Convert to JSON
	return c.JSON(200, dbTorrents)
}
