package qbittorrent

import (
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/kinozal"
	"log"
	"time"
)

// TorrentChecker for checking torrents in the database and on the tracker
func TorrentChecker() {
	for {
		// Get torrent list from database
		dbTorrents, err := database.GetAllRecords(database.DB)
		if err != nil {
			log.Println(err)
			time.Sleep(time.Minute * 5)
			continue
		}

		// Get torrent list from qbittorrent
		qbTorrents, err := GlobalQbittorrentUser.GetTorrentHashList()
		if err != nil {
			log.Println(err)
			time.Sleep(time.Minute * 5)
			continue
		}

		for _, torrent := range dbTorrents {
			// if torrent is not in qbittorrent then add it
			for _, hash := range qbTorrents {
				if hash == torrent.Hash {
					continue
				}
				// Get torrent file from kinozal.tv
				torrentFile, err := kinozal.KinozalUser.DownloadTorrentFile(torrent.Url)
				if err != nil {
					log.Println(err)
					continue
				}

				torrentInfo, err := kinozal.KinozalUser.GetTorrentHash(torrent.Url)
				if err != nil {
					log.Println(err)
					continue
				}

				// Add torrent to qbittorrent
				err = GlobalQbittorrentUser.AddTorrent(torrent.Hash, torrentFile)
				if err != nil {
					log.Println(err)
					continue
				}
				// Update torrent hash in database
				err = database.UpdateRecord(database.DB, torrentInfo)
			}
		}

		time.Sleep(time.Minute * 5)
	}
}
