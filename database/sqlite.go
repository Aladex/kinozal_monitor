package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
)

var DB *sql.DB

// CreateSQLiteDB is a function for creating a database
func CreateSQLiteDB() *sql.DB {
	// Create database folder if not exists
	folderName := "db"
	err := os.MkdirAll(folderName, 0755)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite3", "db/kinozaltv_monitor.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS torrents (
		id INTEGER PRIMARY KEY,
		title TEXT,
		name TEXT,
		hash TEXT,
		url  TEXT
	)`)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func init() {
	DB = CreateSQLiteDB()
}
