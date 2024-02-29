package orm

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"strings"
	"time"
)

const StatsUpdateInterval = 5 * time.Minute

var db *sql.DB

type cachedStats struct {
	stats       map[string]int
	lastUpdated time.Time
}

var signatureStats cachedStats

func GetSignatureStats() (map[string]int, error) {
	if time.Since(signatureStats.lastUpdated) > StatsUpdateInterval {
		err := updateSignatureStats()
		if err != nil {
			return nil, err
		}
	}
	return signatureStats.stats, nil
}

func updateSignatureStats() error {
	db, err := getDBConnection()
	if err != nil {
		return err
	}
	rows, err := db.Query("SELECT email FROM verified_signatures;")
	if err != nil {
		return err
	}
	defer rows.Close()
	var domainCount = make(map[string]int)
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return err
		}

		domain := strings.Split(email, "@")[1]
		domainCount[domain]++
	}
	signatureStats.stats = domainCount
	signatureStats.lastUpdated = time.Now()
	return nil
}

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./signatures.db")
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS verified_signatures (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		ts TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}

	return nil
}

func getDBConnection() (*sql.DB, error) {
	var err error
	if db == nil {
		err = InitDB()
	}

	return db, err
}

func AddSignature(email string) error {
	db, err := getDBConnection()
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT OR IGNORE INTO verified_signatures (email) VALUES (?)", email)
	return err
}
