package database

import (
	"database/sql"
	"kinozaltv_monitor/models"
)

// Torrent is a struct for storing torrent data from the database
type Torrent struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Url        string `json:"url"`
	WatchEvery int    `json:"watch_every"`
}

// GetAllRecords is a function for getting all torrents records from the database
func GetAllRecords(db *sql.DB) (records []Torrent, err error) {
	rows, err := db.Query("SELECT id, title, name, hash, url, watch_every FROM torrents")
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	records = make([]Torrent, 0)
	for rows.Next() {
		var r Torrent
		if scanErr := rows.Scan(&r.ID, &r.Title, &r.Name, &r.Hash, &r.Url, &r.WatchEvery); scanErr != nil {
			return nil, scanErr
		}
		records = append(records, r)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// CreateOrUpdateRecord is a function for creating or updating a torrent record in the database
func CreateOrUpdateRecord(db *sql.DB, torrentInfo models.Torrent) error {
	// Check if torrent exists in the sqlite database
	var hash string
	err := db.QueryRow("SELECT hash FROM torrents WHERE url = ?", torrentInfo.Url).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			// Add torrent to the database
			err = AddRecord(db, torrentInfo)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	} else {
		// Update torrent in the database
		err = UpdateRecord(db, torrentInfo)
		if err != nil {
			return err
		}
		return nil
	}
}

// AddRecord is a function for adding a torrent record to the database
func AddRecord(db *sql.DB, torrentInfo models.Torrent) error {
	_, err := db.Exec("INSERT INTO torrents (title, name, hash, url) VALUES (?, ?, ?, ?)", torrentInfo.Title, torrentInfo.Name, torrentInfo.Hash, torrentInfo.Url)
	if err != nil {
		return err
	}
	return nil
}

// DeleteRecord is a function for deleting a torrent record from the database
func DeleteRecord(db *sql.DB, url string) error {
	_, err := db.Exec("DELETE FROM torrents WHERE url = ?", url)
	if err != nil {
		return err
	}

	return nil
}

// UpdateRecord is a function for updating hash and title for a torrent record in the database
func UpdateRecord(db *sql.DB, torrentInfo models.Torrent) error {
	_, err := db.Exec("UPDATE torrents SET hash = ?, title = ? WHERE url = ?", torrentInfo.Hash, torrentInfo.Title, torrentInfo.Url)
	if err != nil {
		return err
	}

	return nil
}

// SetWatchFlag is a function for setting watch_it flag for a torrent record in the database
func SetWatchFlag(db *sql.DB, url string, watchPeriod int) error {
	_, err := db.Exec("UPDATE torrents SET watch_every = ? WHERE url = ?", watchPeriod, url)
	if err != nil {
		return err
	}
	return nil
}
