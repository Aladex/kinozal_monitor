package qbittorrent

import (
	"kinozaltv_monitor/config"
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/kinozal"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/telegram"
	"time"
)

var log = logger.New("qbittorrent")

// TorrentChecker for checking torrents in the database and on the tracker
func TorrentChecker() {
	// print Checker started
	log.Info("info", "Checker started", nil)
	for {
		// Get torrent list from database
		dbTorrents, err := database.GetAllRecords(database.DB)
		if err != nil {
			log.Error("get_db_records", err.Error(), nil)
			time.Sleep(time.Minute * 5)
			continue
		}

		// Get torrent list from qbittorrent
		qbTorrents, err := GlobalQbittorrentUser.GetTorrentHashList()
		if err != nil {
			log.Error("get_qb_torrents", err.Error(), nil)
			handleQbittorrentError(err)
			continue
		}

		for _, dbTorrent := range dbTorrents {
			qbTorrent := Torrent{
				Hash: dbTorrent.Hash,
				Name: dbTorrent.Name,
				Url:  dbTorrent.Url,
			}
			if !contains(qbTorrents, dbTorrent.Hash) {
				// If the torrent is not in qbittorrent, add it and send notification
				addTorrentToQbittorrent(qbTorrent)
				continue
			}

			// Get torrent info from kinozal.tv
			torrentInfo, err := kinozal.KinozalUser.GetTorrentHash(dbTorrent.Url)
			if err != nil {
				log.Error("get_torrent_info", err.Error(), nil)
				continue
			}

			// If hash is not equal then update torrent
			if torrentInfo.Hash != dbTorrent.Hash {
				updateTorrentInQbittorrent(qbTorrent, torrentInfo)
			}
		}

		time.Sleep(time.Minute * 5)
	}
}

func handleQbittorrentError(err error) {
	// If returned Forbidden then try to login
	if err.Error() == "Forbidden" {
		// Drop current session
		err = GlobalQbittorrentUser.DropLoginSession()
		if err != nil {
			log.Error("drop_login_session", err.Error(), nil)
			time.Sleep(time.Minute * 5)
			return
		}

		// Login
		err = GlobalQbittorrentUser.Login()
		if err != nil {
			log.Error("qbittorrent_login", err.Error(), nil)
			time.Sleep(time.Minute * 5)
			return
		}
	}

	time.Sleep(time.Minute * 5)
}

func contains(s []Torrent, e string) bool {
	for _, a := range s {
		if a.Hash == e {
			return true
		}
	}
	return false
}

func addTorrentToQbittorrent(dbTorrent Torrent) {
	// Get torrent file from kinozal.tv
	torrentFile, err := kinozal.KinozalUser.DownloadTorrentFile(dbTorrent.Url)
	if err != nil {
		log.Error("download_torrent_file", err.Error(), map[string]string{
			"torrent_url": dbTorrent.Url,
		})
		return
	}

	// Add torrent to qbittorrent
	err = GlobalQbittorrentUser.AddTorrent(dbTorrent.Hash, torrentFile)
	if err != nil {
		log.Error("add_torrent", err.Error(), nil)
		return
	}

	// Get torrent info from kinozal.tv
	torrentInfo, err := kinozal.KinozalUser.GetTorrentHash(dbTorrent.Url)
	if err != nil {
		log.Error("get_torrent_info", err.Error(), nil)
		return
	}

	// Send notification to telegram
	err = telegram.SendTorrentAction("added", config.GlobalConfig.TelegramToken, torrentInfo)
	if err != nil {
		log.Error("send_telegram_notification", err.Error(), nil)
		return
	}
}

func updateTorrentInQbittorrent(dbTorrent Torrent, torrentInfo kinozal.KinozalTorrent) {
	// Remove torrent from qbittorrent
	err := GlobalQbittorrentUser.DeleteTorrent(dbTorrent.Hash, false)
	if err != nil {
		log.Error("delete_torrent", err.Error(), nil)
		return
	}

	// Send notification to telegram
	err = telegram.SendTorrentAction("updated", config.GlobalConfig.TelegramToken, torrentInfo)
	if err != nil {
		log.Error("send_telegram_notification", err.Error(), nil)
		return
	}

	// Update torrent hash in database
	err = database.UpdateRecord(database.DB, torrentInfo)
	if err != nil {
		log.Error("update_db_record", err.Error(), nil)
		return
	}

	// Add the updated torrent back to qbittorrent
	addTorrentToQbittorrent(dbTorrent)
}
