package main

import (
    "log"
    "net/http"
    "os"
    "time"

    "github.com/smeetnagda/vmshare/internal/server"
)

func main() {
    // 1) Initialize (and migrate) the SQLite database
    dbPath := "data/vmrental.db"
    db, err := server.NewDB(dbPath)
    if err != nil {
        log.Fatalf("Failed to open database %q: %v", dbPath, err)
    }
    defer db.Close()
    log.Printf("✅ Database ready: %s", dbPath)

    // 2) Periodically clean up expired rentals
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        for range ticker.C {
            n, err := server.DeleteExpiredRentals(db)
            if err != nil {
                log.Printf("Error deleting expired rentals: %v", err)
            } else if n > 0 {
                log.Printf("Cleaned up %d expired rentals", n)
            }
        }
    }()

    // 3) Register HTTP handlers
    mux := http.NewServeMux()
    mux.HandleFunc("/rentals", server.HandleCreateRental(db))  // POST
    mux.HandleFunc("/rentals/", server.HandleDeleteRental(db)) // DELETE

    // Optional health check
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.Write([]byte("OK"))
    })

    // 4) Start HTTP server
    addr := os.Getenv("HTTP_ADDR")
    if addr == "" {
        addr = ":8080"
    }
    log.Printf("🚀 Coordinator listening on %s …", addr)
    if err := http.ListenAndServe(addr, mux); err != nil {
        log.Fatalf("HTTP server error: %v", err)
    }
}

