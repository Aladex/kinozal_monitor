package qbittorrent

import (
	"kinozaltv_monitor/config"
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/kinozal"
	"kinozaltv_monitor/telegram"
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

			// If returned Forbidden then try to login
			if err.Error() == "Forbidden" {
				// Drop current session
				err = GlobalQbittorrentUser.DropLoginSession()
				if err != nil {
					log.Println(err)
					time.Sleep(time.Minute * 5)
					continue
				}

				// Login
				err = GlobalQbittorrentUser.Login()
				if err != nil {
					log.Println(err)
					time.Sleep(time.Minute * 5)
					continue
				}
			}

			time.Sleep(time.Minute * 5)
			continue
		}

		for _, dbTorrent := range dbTorrents {
			// if torrent is not in qbittorrent then add it
			for _, qbTorrent := range qbTorrents {
				// if torrent is in qbittorrent then check it
				if qbTorrent.Hash == dbTorrent.Hash {
					// Get torrent info from kinozal.tv
					torrentInfo, err := kinozal.KinozalUser.GetTorrentHash(dbTorrent.Url)
					if err != nil {
						log.Println(err)
						continue
					}

					// If hash is not equal then update torrent
					if torrentInfo.Hash != dbTorrent.Hash {

						// Remove torrent from qbittorrent
						err = GlobalQbittorrentUser.DeleteTorrent(dbTorrent.Hash, false)
						if err != nil {
							log.Println(err)
							continue
						}

						// Send notification to telegram
						err = telegram.SendTorrentAction("updated", config.GlobalConfig.TelegramToken, torrentInfo)
						if err != nil {
							log.Println(err)
							continue
						}
					}
					// Get torrent file from kinozal.tv
					torrentFile, err := kinozal.KinozalUser.DownloadTorrentFile(dbTorrent.Url)
					if err != nil {
						log.Println(err)
						continue
					} else {
						// Add torrent to qbittorrent
						err = GlobalQbittorrentUser.AddTorrent(dbTorrent.Hash, torrentFile)
						if err != nil {
							log.Println(err)
							continue
						} else {
							// Send notification to telegram
							err = telegram.SendTorrentAction("added", config.GlobalConfig.TelegramToken, torrentInfo)
							if err != nil {
								log.Println(err)
								continue
							}
						}
					}

					torrentInfo, err = kinozal.KinozalUser.GetTorrentHash(dbTorrent.Url)
					if err != nil {
						log.Println(err)
						continue
					}
					// Get title from original url
					title, err := kinozal.KinozalUser.GetTitleFromUrl(dbTorrent.Url)
					if err != nil {
						log.Println(err)
						continue
					}

					// Set title to torrentInfo
					torrentInfo.Title = title

					// Update torrent hash in database
					err = database.UpdateRecord(database.DB, torrentInfo)
				}
			}
		}

		time.Sleep(time.Minute * 5)
	}
}
