package qbittorrent

import (
	"kinozaltv_monitor/common"
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/kinozal"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/telegram"
	"time"
)

var log = logger.New("qbittorrent")
var kzUser = kinozal.KinozalUser

// torrentAdder
func torrentAdder(torrentData common.TorrentData, wsMsg chan string) {
	torrentInfo, err := kzUser.GetTorrentHash(torrentData.Url)
	if err != nil {
		log.Error("get_torrent_info", err.Error(), nil)
		// Return 500 Internal Server Error
		wsMsg <- "500"
		return
	}
	// Check if torrent exists in qbittorrent
	torrentHashList, err := GlobalQbittorrentUser.GetTorrentHashList()
	if err != nil {
		log.Error("get_qb_torrents", err.Error(), nil)
		wsMsg <- "500"
		return
	}

	// Get title from original url
	title, err := kzUser.GetTitleFromUrl(torrentData.Url)
	if err != nil {
		log.Error("get_title_from_url", err.Error(), nil)
		wsMsg <- "500"
		return
	}

	// Set title to torrentInfo
	torrentInfo.Title = title

	// Add torrent to database
	err = database.CreateOrUpdateRecord(database.DB, torrentInfo)
	if err != nil {
		log.Error("create_or_update_record", err.Error(), nil)
		wsMsg <- "500"
		return
	}

	for _, hash := range torrentHashList {
		if hash.Hash == torrentInfo.Hash {
			// Torrent already exists in qbittorrent
			wsMsg <- "added"
			return
		}
	}

	// Add torrent to qbittorrent by addToQbittorrent function

	// Create qbitTorrent struct
	qbTorrent := Torrent{
		Hash:     torrentInfo.Hash,
		Name:     torrentInfo.Title,
		Url:      torrentInfo.Url,
		SavePath: torrentData.DownloadPath,
	}
	go func() {
		// Add torrent to qbittorrent
		addTorrentToQbittorrent(qbTorrent, true)

		// Send websocket message about adding torrent
		log.Info("info", "Torrent added", map[string]string{
			"torrent_url": torrentData.Url,
		})
		wsMsg <- "added"
	}()
}

func torrentWorker() {
	// Get torrent list from database
	dbTorrents, err := database.GetAllRecords(database.DB)
	if err != nil {
		log.Error("get_db_records", err.Error(), nil)
		return
	}

	// Get torrent list from qbittorrent
	qbTorrents, err := GlobalQbittorrentUser.GetTorrentHashList()
	if err != nil {
		log.Error("get_qb_torrents", err.Error(), nil)
		handleQbittorrentError(err)
		return
	}

	for _, dbTorrent := range dbTorrents {
		qbTorrent := Torrent{
			Hash:  dbTorrent.Hash,
			Title: dbTorrent.Title,
			Name:  dbTorrent.Name,
			Url:   dbTorrent.Url,
		}
		if !contains(qbTorrents, dbTorrent.Hash) {
			if !addTorrentToQbittorrent(qbTorrent, true) {
				continue
			}
		}

		// Get torrent info from kinozal.tv
		log.Info("info", "Get torrent info from kinozal.tv", map[string]string{
			"torrent_url":  dbTorrent.Url,
			"torrent_hash": dbTorrent.Hash,
			"reason":       "check if torrent exists in kinozal.tv",
		})
		torrentInfo, err := kzUser.GetTorrentHash(dbTorrent.Url)
		if err != nil {
			log.Error("get_torrent_info", err.Error(), nil)
			continue
		}

		// If hash is not equal then update torrent
		if torrentInfo.Hash != dbTorrent.Hash {
			// Update title of torrent
			torrentInfo.Title, err = kzUser.GetTitleFromUrl(dbTorrent.Url)
			if err != nil {
				log.Error("get_title_from_url", err.Error(), nil)
				// Set title from database
				torrentInfo.Title = dbTorrent.Title
			}
			if !updateTorrentInQbittorrent(qbTorrent, torrentInfo) {
				continue
			}
		}
	}
}

// TorrentChecker for checking torrents in the database and on the tracker
func TorrentChecker() {
	log.Info("info", "Checker started", nil)

	torrentWorker()

	ticker := time.NewTicker(time.Minute * 10)
	for {
		select {
		case <-ticker.C:
			torrentWorker()
		}
	}
}

// WsMessageHandler for handling websocket messages
func WsMessageHandler(wsMsg chan string, torrentData chan common.TorrentData) {
	log.Info("info", "Websocket handler started", nil)
	for {
		select {
		case torrentUrl := <-torrentData:
			log.Info("info", "URL received", map[string]string{
				"torrent_url": torrentUrl.Url,
			})
			go torrentAdder(torrentUrl, wsMsg)
		}
	}
}

func handleQbittorrentError(err error) {
	if err.Error() == "Forbidden" {
		err = GlobalQbittorrentUser.DropLoginSession()
		if err != nil {
			log.Error("drop_login_session", err.Error(), nil)
		}

		err = GlobalQbittorrentUser.Login()
		if err != nil {
			log.Error("qbittorrent_login", err.Error(), nil)
		}
	}
}

func contains(s []Torrent, e string) bool {
	for _, a := range s {
		if a.Hash == e {
			return true
		}
	}
	return false
}

func addTorrentToQbittorrent(dbTorrent Torrent, sendTgMessage bool) bool {
	torrentFile, err := kzUser.DownloadTorrentFile(dbTorrent.Url)
	if err != nil {
		log.Info("download_torrent_file", err.Error(), map[string]string{
			"torrent_url": dbTorrent.Url,
			"reason":      "torrent file not found",
			"result":      "try to add by magnet link",
		})

		// Add torrent my magnet link
		err = GlobalQbittorrentUser.AddTorrentByMagnet(dbTorrent.Hash, dbTorrent.SavePath)
		if err != nil {
			log.Error("add_torrent_by_magnet", err.Error(), nil)
			return false
		}
	} else {
		err = GlobalQbittorrentUser.AddTorrent(dbTorrent.Hash, dbTorrent.SavePath, torrentFile)
		if err != nil {
			log.Error("add_torrent", err.Error(), nil)
			return false
		}
	}

	torrentInfo, err := kzUser.GetTorrentHash(dbTorrent.Url)
	if err != nil {
		log.Error("get_torrent_info", err.Error(), nil)
		return false
	}

	// Get title from kinozal.tv
	torrentInfo.Title, err = kzUser.GetTitleFromUrl(dbTorrent.Url)
	if err != nil {
		log.Error("get_title_from_url", err.Error(), nil)
		return false
	}

	if sendTgMessage {
		err = telegram.SendTorrentAction("added", globalConfig.TelegramToken, torrentInfo)
		if err != nil {
			log.Error("send_telegram_notification", err.Error(), nil)
		}

	}

	return true
}

func updateTorrentInQbittorrent(dbTorrent Torrent, torrentInfo kinozal.KinozalTorrent) bool {
	err := GlobalQbittorrentUser.DeleteTorrent(dbTorrent.Hash, false)
	if err != nil {
		log.Error("delete_torrent", err.Error(), nil)
		return false
	}

	err = telegram.SendTorrentAction("updated", globalConfig.TelegramToken, torrentInfo)
	if err != nil {
		log.Error("send_telegram_notification", err.Error(), nil)
		return false
	}

	err = database.UpdateRecord(database.DB, torrentInfo)
	if err != nil {
		log.Error("update_db_record", err.Error(), nil)
		return false
	}

	return addTorrentToQbittorrent(dbTorrent, false)
}
