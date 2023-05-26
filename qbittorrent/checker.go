package qbittorrent

import (
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/kinozal"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/telegram"
	"time"
)

var log = logger.New("qbittorrent")
var kzUser = kinozal.KinozalUser

func torrentWorker() {
	for {
		// Get torrent list from database
		dbTorrents, err := database.GetAllRecords(database.DB)
		if err != nil {
			log.Error("get_db_records", err.Error(), nil)
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
				if !addTorrentToQbittorrent(qbTorrent) {
					continue
				}
			}

			// Get torrent info from kinozal.tv
			torrentInfo, err := kzUser.GetTorrentHash(dbTorrent.Url)
			if err != nil {
				log.Error("get_torrent_info", err.Error(), nil)
				continue
			}

			// If hash is not equal then update torrent
			if torrentInfo.Hash != dbTorrent.Hash {
				if !updateTorrentInQbittorrent(qbTorrent, torrentInfo) {
					continue
				}
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

func addTorrentToQbittorrent(dbTorrent Torrent) bool {
	torrentFile, err := kzUser.DownloadTorrentFile(dbTorrent.Url)
	if err != nil {
		log.Info("download_torrent_file", err.Error(), map[string]string{
			"torrent_url": dbTorrent.Url,
			"reason":      "torrent file not found",
			"result":      "try to add by magnet link",
		})

		// Add torrent my magnet link
		err = GlobalQbittorrentUser.AddTorrentByMagnet(dbTorrent.Hash)
		if err != nil {
			log.Error("add_torrent_by_magnet", err.Error(), nil)
			return false
		}
	} else {
		err = GlobalQbittorrentUser.AddTorrent(dbTorrent.Hash, torrentFile)
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

	err = telegram.SendTorrentAction("added", globalConfig.TelegramToken, torrentInfo)
	if err != nil {
		log.Error("send_telegram_notification", err.Error(), nil)
		return false
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

	return addTorrentToQbittorrent(dbTorrent)
}
