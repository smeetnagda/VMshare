package server

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// NewDB opens (or creates) the SQLite file at dbPath, runs migrations, and returns the *sql.DB.
func NewDB(dbPath string) (*sql.DB, error) {
	// Ensure the data directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory %%s: %%v", dir, err)
	}

	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %v", err)
	}

	// Read and execute migrations
	schemaPath := filepath.Join("migrations", "schema.sql")
	schema, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("read schema.sql: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		db.Close()
		return nil, fmt.Errorf("exec migrations: %v", err)
	}

	return db, nil
}
