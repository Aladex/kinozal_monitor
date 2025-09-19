package qbittorrent

import (
	"context"
	"fmt"
	"kinozaltv_monitor/common"
	"kinozaltv_monitor/config"
	"kinozaltv_monitor/database"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/models"
	"kinozaltv_monitor/telegram"
	"strconv"
	"time"
)

var log = logger.New("qbittorrent")
var kzUser = models.KinozalUser
var rtUser = models.RTUser
var globalConfig = config.GlobalConfig

type TorrentWatcher struct {
	cancel     context.CancelFunc
	watchEvery int
}

// torrentAdder
func torrentAdder(torrentData common.TorrentData, wsMsg chan string) {
	// Get the appropriate tracker based on URL
	tracker, err := models.GlobalTrackerManager.GetTrackerByURL(torrentData.Url)
	if err != nil {
		log.Error("get_tracker", "Error while getting tracker for URL", map[string]string{"error": err.Error(), "url": torrentData.Url})
		wsMsg <- "500"
		return
	}

	torrentInfo, err := tracker.GetTorrentHash(torrentData.Url)
	if err != nil {
		log.Error("get_torrent_info", "Error while getting torrent info", map[string]string{"error": err.Error()})
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
	title, err := tracker.GetTitleFromUrl(torrentData.Url)
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

func torrentWorker(ctx context.Context, dbTorrent database.Torrent) {
	log.Info("info", "Torrent worker started", map[string]string{
		"torrent_url":  dbTorrent.Url,
		"torrent_hash": dbTorrent.Hash,
		"watch_every":  fmt.Sprintf("%d minutes", dbTorrent.WatchEvery),
	})

	// Check torrent
	updatedTorrent, err := torrentChecker(dbTorrent)
	if err != nil {
		log.Error("torrent_checker", err.Error(), nil)
	} else {
		// Update torrent
		dbTorrent = updatedTorrent
	}

	// Create ticker for checking torrent every watch interval
	ticker := time.NewTicker(time.Duration(dbTorrent.WatchEvery) * time.Minute)
	defer ticker.Stop() // Important to stop the ticker when the function exits

	for {
		select {
		case <-ticker.C:
			// Check torrent
			updatedTorrent, err := torrentChecker(dbTorrent)
			if err != nil {
				log.Error("torrent_checker", err.Error(), nil)
			} else {
				// Update torrent
				dbTorrent = updatedTorrent
			}
		case <-ctx.Done():
			// Context cancelled, stop the worker
			log.Info("info", "Torrent worker cancelled", map[string]string{
				"torrent_url":  dbTorrent.Url,
				"torrent_hash": dbTorrent.Hash,
			})
			return
		}
	}
}

func torrentChecker(dbTorrent database.Torrent) (database.Torrent, error) {
	// Get torrent list from qbittorrent
	qbTorrents, err := GlobalQbittorrentUser.GetTorrentHashList()
	if err != nil {
		log.Error("get_qb_torrents", err.Error(), nil)
		handleQbittorrentError(err)
		return dbTorrent, err
	}

	qbTorrent := Torrent{
		Hash:     dbTorrent.Hash,
		Title:    dbTorrent.Title,
		Name:     dbTorrent.Name,
		Url:      dbTorrent.Url,
		SavePath: "", // Will be populated if needed
	}

	// If torrent does not exist in qbittorrent
	if !contains(qbTorrents, dbTorrent.Hash) {
		log.Info("torrent_missing", "Torrent not found in qBittorrent, re-adding", map[string]string{
			"torrent_url":  dbTorrent.Url,
			"torrent_hash": dbTorrent.Hash,
		})
		if !addTorrentToQbittorrent(qbTorrent, true) {
			return dbTorrent, fmt.Errorf("torrent not added to qbittorrent")
		}
	} else {
		// Get the appropriate tracker based on URL
		tracker, err := models.GlobalTrackerManager.GetTrackerByURL(dbTorrent.Url)
		if err != nil {
			log.Error("get_tracker", "Error while getting tracker for URL", map[string]string{"error": err.Error(), "url": dbTorrent.Url})
			return dbTorrent, err
		}

		// Get torrent info from tracker
		log.Info("info", "Checking for torrent updates from tracker", map[string]string{
			"torrent_url":  dbTorrent.Url,
			"torrent_hash": dbTorrent.Hash,
			"reason":       "periodic update check",
		})
		torrentInfo, err := tracker.GetTorrentHash(dbTorrent.Url)
		if err != nil {
			log.Error("get_torrent_info", "Error while getting torrent info from tracker", map[string]string{"error": err.Error()})
			return dbTorrent, err
		}

		// If hash is not equal then update torrent
		if torrentInfo.Hash != dbTorrent.Hash {
			log.Info("torrent_update_detected", "Torrent hash changed, updating", map[string]string{
				"torrent_url": dbTorrent.Url,
				"old_hash":    dbTorrent.Hash,
				"new_hash":    torrentInfo.Hash,
			})

			// Get updated title from tracker
			torrentInfo.Title, err = tracker.GetTitleFromUrl(dbTorrent.Url)
			if err != nil {
				log.Error("get_title_from_url", "Error getting updated title", map[string]string{"error": err.Error()})
				// Use existing title as fallback
				torrentInfo.Title = dbTorrent.Title
			}

			// Set the URL for the torrent info
			torrentInfo.Url = dbTorrent.Url

			// Get current save path before deletion
			savePath, err := GlobalQbittorrentUser.GetDownloadPathByHash(dbTorrent.Hash)
			if err != nil {
				log.Error("get_download_path", "Error getting download path, using default", map[string]string{"error": err.Error()})
				savePath = "/downloads" // fallback path
			}
			qbTorrent.SavePath = savePath

			if !updateTorrentInQbittorrent(qbTorrent, torrentInfo) {
				log.Error("update_torrent_in_qbittorrent", "Failed to update torrent in qBittorrent", map[string]string{
					"torrent_url": dbTorrent.Url,
					"old_hash":    dbTorrent.Hash,
					"new_hash":    torrentInfo.Hash,
				})
				return dbTorrent, fmt.Errorf("torrent not updated in qbittorrent")
			}

			// Update the database torrent record with new hash and title
			dbTorrent.Hash = torrentInfo.Hash
			dbTorrent.Title = torrentInfo.Title
			dbTorrent.Name = torrentInfo.Title

			log.Info("torrent_updated_successfully", "Torrent updated successfully", map[string]string{
				"torrent_url": dbTorrent.Url,
				"new_hash":    dbTorrent.Hash,
				"new_title":   dbTorrent.Title,
			})
		} else {
			log.Info("torrent_up_to_date", "Torrent is up to date", map[string]string{
				"torrent_url":  dbTorrent.Url,
				"torrent_hash": dbTorrent.Hash,
			})
		}
	}

	return dbTorrent, nil
}

// createOrUpdateWatcher creates or updates watcher for torrent
func createOrUpdateWatcher(dbTorrent database.Torrent, torrentWatchers map[int]*TorrentWatcher) {
	// Create context for watcher
	ctx, cancel := context.WithCancel(context.Background())
	// Create or update watcher
	torrentWatchers[dbTorrent.ID] = &TorrentWatcher{
		cancel:     cancel,
		watchEvery: dbTorrent.WatchEvery,
	}
	go torrentWorker(ctx, dbTorrent)
	log.Info("info", "Torrent watcher created or updated", map[string]string{
		"torrent_url":  dbTorrent.Url,
		"torrent_hash": dbTorrent.Hash,
		"watch_every":  strconv.Itoa(dbTorrent.WatchEvery),
	})
}

func containsTorrent(dbTorrents []database.Torrent, id int) bool {
	for _, a := range dbTorrents {
		if a.ID == id {
			return true
		}
	}
	return false
}

func deleteRemovedTorrents(dbTorrents []database.Torrent, torrentWatchers map[int]*TorrentWatcher) {
	// Find ID of torrents that are not in database
	var torrentsToDelete []int
	for id := range torrentWatchers {
		if !containsTorrent(dbTorrents, id) {
			torrentsToDelete = append(torrentsToDelete, id)
		}
	}

	// Delete watchers
	for _, id := range torrentsToDelete {
		torrentWatchers[id].cancel()
		delete(torrentWatchers, id)
		log.Info("info", "Torrent watcher deleted", map[string]string{
			"torrent_id": strconv.Itoa(id),
		})
	}
}

// TorrentChecker checks torrents in database and qbittorrent
func TorrentChecker() {
	log.Info("info", "Checker started", nil)
	// Get torrent list from database every 5 minutes
	// And create watcher for every torrent if it has watch interval
	torrentWatchers := make(map[int]*TorrentWatcher)

	for {
		// Get torrent list from database
		dbTorrents, err := database.GetAllRecords(database.DB)
		if err != nil {
			log.Error("get_db_records", err.Error(), nil)
			return
		}

		// Delete removed torrents
		deleteRemovedTorrents(dbTorrents, torrentWatchers)

		// Iterate over torrents and create watcher for every torrent with watch interval
		for _, dbTorrent := range dbTorrents {
			if dbTorrent.WatchEvery > 0 {
				// Check if watcher already exists
				if watcher, ok := torrentWatchers[dbTorrent.ID]; ok {
					// If watch interval changed then delete watcher and create new one
					if watcher.watchEvery != dbTorrent.WatchEvery {
						watcher.cancel()

						// If watch interval equals to 0 then delete watcher
						if dbTorrent.WatchEvery == 0 {
							delete(torrentWatchers, dbTorrent.ID)
							log.Info("info", "Torrent watcher deleted", map[string]string{
								"torrent_url":  dbTorrent.Url,
								"torrent_hash": dbTorrent.Hash,
							})
						} else {
							createOrUpdateWatcher(dbTorrent, torrentWatchers)
						}
					}
				} else {
					createOrUpdateWatcher(dbTorrent, torrentWatchers)
				}
			}
		}
		// Sleep for 5 seconds
		time.Sleep(5 * time.Second)
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

func kinozalAction(dbTorrent Torrent) (models.Torrent, error) {
	// Get the Kinozal tracker from tracker manager
	tracker, err := models.GlobalTrackerManager.GetTracker("kinozal")
	if err != nil {
		log.Error("get_tracker", "Error getting Kinozal tracker from manager", map[string]string{"error": err.Error()})
		return models.Torrent{}, err
	}

	torrentFile, err := tracker.DownloadTorrentFile(dbTorrent.Url)
	if err != nil {
		log.Info("download_torrent_file", "Error downloading torrent file", map[string]string{
			"torrent_url": dbTorrent.Url,
			"reason":      "torrent file not found",
			"result":      "try to add by magnet link",
		})

		// Add torrent by magnet link
		addErr := GlobalQbittorrentUser.AddTorrentByMagnet(dbTorrent.Hash, dbTorrent.SavePath)
		if addErr != nil {
			log.Error("add_torrent_by_magnet", "Error adding torrent by magnet link", map[string]string{"error": addErr.Error()})
			return models.Torrent{}, addErr
		}
	} else {
		addErr := GlobalQbittorrentUser.AddTorrent(dbTorrent.Hash, dbTorrent.SavePath, torrentFile)
		if addErr != nil {
			log.Error("add_torrent", "Error adding torrent", map[string]string{"error": addErr.Error()})
			return models.Torrent{}, addErr
		}
	}

	torrentInfo, err := tracker.GetTorrentHash(dbTorrent.Url)
	if err != nil {
		log.Error("get_torrent_info", "Error getting torrent info", map[string]string{"error": err.Error()})
		return models.Torrent{}, err
	}

	// Get title from kinozal.tv
	title, err := tracker.GetTitleFromUrl(dbTorrent.Url)
	if err != nil {
		log.Error("get_title_from_url", "Error getting title from URL", map[string]string{"error": err.Error()})
		return models.Torrent{}, err
	}

	torrentInfo.Title = title

	return torrentInfo, nil
}

func addTorrentToQbittorrent(dbTorrent Torrent, sendTgMessage bool) bool {
	// Check what torrent tracker is in the URL
	trackerDomain := common.GetTrackerDomain(dbTorrent.Url)
	var torrentInfo models.Torrent
	var err error
	var torrentData []byte = nil

	switch trackerDomain {
	case "kinozal.tv":
		torrentInfo, err = kinozalAction(dbTorrent)
		if err != nil {
			log.Error("kinozal_action", err.Error(), nil)
			return false
		}
	case "rutracker.org":
		// Get the RuTracker tracker from tracker manager
		tracker, err := models.GlobalTrackerManager.GetTracker("rutracker")
		if err != nil {
			log.Error("get_tracker", "Error getting RuTracker tracker from manager", map[string]string{"error": err.Error()})
			return false
		}

		// First, get the hash without downloading the torrent again if possible
		torrentInfo, err = tracker.GetTorrentHash(dbTorrent.Url)
		if err != nil {
			log.Error("get_rt_torrent_hash", err.Error(), nil)
			return false
		}

		// Get title from rutracker.org
		title, err := tracker.GetTitleFromUrl(dbTorrent.Url)
		if err != nil {
			log.Error("get_title_from_url", "Error getting title from URL", map[string]string{"error": err.Error()})
			// If we can't get the title, use the hash as a fallback
			title = dbTorrent.Hash
		}

		// Set the title
		torrentInfo.Title = title

		// Download the torrent file directly
		torrentData, err = tracker.DownloadTorrentFile(dbTorrent.Url)
		if err != nil {
			log.Error("download_torrent", "Error downloading torrent file", map[string]string{"error": err.Error()})
			return false
		}

		// Add the torrent to qBittorrent using the downloaded file
		addErr := GlobalQbittorrentUser.AddTorrent(dbTorrent.Hash, dbTorrent.SavePath, torrentData)
		if addErr != nil {
			log.Error("add_torrent", "Error adding torrent", map[string]string{"error": addErr.Error()})
			return false
		}
	}

	// Save torrent information to database
	err = database.CreateOrUpdateRecord(database.DB, torrentInfo)
	if err != nil {
		log.Error("create_or_update_record", "Error saving torrent info to database", map[string]string{"error": err.Error()})
	}

	if sendTgMessage {
		err := telegram.SendTorrentAction("added", globalConfig.TelegramToken, torrentInfo)
		if err != nil {
			log.Error("send_telegram_notification", err.Error(), nil)
		}
	}
	return true
}

func updateTorrentInQbittorrent(dbTorrent Torrent, torrentInfo models.Torrent) bool {
	log.Info("update_torrent_start", "Starting torrent update process", map[string]string{
		"torrent_url": dbTorrent.Url,
		"old_hash":    dbTorrent.Hash,
		"new_hash":    torrentInfo.Hash,
	})

	// Find save path of torrent before deletion
	savePath, err := GlobalQbittorrentUser.GetDownloadPathByHash(dbTorrent.Hash)
	if err != nil {
		log.Error("get_download_path", "Error getting download path for torrent", map[string]string{
			"error":        err.Error(),
			"torrent_hash": dbTorrent.Hash,
		})
		// Use the existing save path as fallback
		if dbTorrent.SavePath != "" {
			savePath = dbTorrent.SavePath
		} else {
			savePath = "/downloads" // default fallback
		}
	}
	dbTorrent.SavePath = savePath

	// Delete old torrent from qBittorrent (keep files)
	err = GlobalQbittorrentUser.DeleteTorrent(dbTorrent.Hash, false)
	if err != nil {
		log.Error("delete_torrent", "Error deleting old torrent from qBittorrent", map[string]string{
			"error":        err.Error(),
			"torrent_hash": dbTorrent.Hash,
		})
		return false
	}

	log.Info("old_torrent_deleted", "Old torrent deleted from qBittorrent", map[string]string{
		"old_hash": dbTorrent.Hash,
	})

	// Update database record with new torrent info
	err = database.UpdateRecord(database.DB, torrentInfo)
	if err != nil {
		log.Error("update_db_record", "Error updating torrent record in database", map[string]string{
			"error":    err.Error(),
			"new_hash": torrentInfo.Hash,
		})
		return false
	}

	log.Info("database_updated", "Database record updated with new torrent info", map[string]string{
		"new_hash": torrentInfo.Hash,
	})

	// Create new torrent struct with updated info for adding to qBittorrent
	newTorrent := Torrent{
		Hash:     torrentInfo.Hash,
		Name:     torrentInfo.Title,
		Title:    torrentInfo.Title,
		Url:      torrentInfo.Url,
		SavePath: savePath,
	}

	// Add updated torrent to qBittorrent
	if !addTorrentToQbittorrent(newTorrent, false) {
		log.Error("add_updated_torrent", "Error adding updated torrent to qBittorrent", map[string]string{
			"new_hash": torrentInfo.Hash,
		})
		return false
	}

	// Send Telegram notification about the update
	err = telegram.SendTorrentAction("updated", globalConfig.TelegramToken, torrentInfo)
	if err != nil {
		log.Error("send_telegram_notification", "Error sending Telegram notification", map[string]string{
			"error": err.Error(),
		})
		// Don't return false here as the update was successful, just notification failed
	}

	log.Info("torrent_update_completed", "Torrent update process completed successfully", map[string]string{
		"torrent_url": torrentInfo.Url,
		"old_hash":    dbTorrent.Hash,
		"new_hash":    torrentInfo.Hash,
		"title":       torrentInfo.Title,
	})

	return true
}
