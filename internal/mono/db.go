package mono

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT UNIQUE NOT NULL,
    docker_project TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

type DB struct {
	conn *sql.DB
	path string
}

func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	monoDir := filepath.Join(home, ".mono")
	if err := os.MkdirAll(monoDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ~/.mono directory: %w", err)
	}

	return filepath.Join(monoDir, "state.db"), nil
}

func OpenDB() (*DB, error) {
	dbPath, err := DBPath()
	if err != nil {
		return nil, err
	}

	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := &DB{conn: conn, path: dbPath}

	if err := db.Initialize(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Initialize() error {
	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}
