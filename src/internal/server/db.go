package server

import (
    "database/sql"
    "fmt"
    "io/ioutil"
    "path/filepath"

    _ "github.com/mattn/go-sqlite3"
)

// NewDB opens (or creates) the SQLite file at path, runs migrations, and returns the *sql.DB.
func NewDB(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }

    // Read and execute migrations
    schemaPath := filepath.Join("migrations", "schema.sql")
    sqlBytes, err := ioutil.ReadFile(schemaPath)
    if err != nil {
        return nil, fmt.Errorf("read schema.sql: %w", err)
    }

    if _, err := db.Exec(string(sqlBytes)); err != nil {
        return nil, fmt.Errorf("exec migrations: %w", err)
    }

    return db, nil
}

