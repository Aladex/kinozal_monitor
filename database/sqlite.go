package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

var DB *sql.DB

// CreateSQLiteDB is a function for creating a database
func CreateSQLiteDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./kinozal.db")
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
