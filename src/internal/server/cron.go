package server

import (
    "database/sql"
    "log"
    "time"
)

// StartExpiredRentalCleanup kicks off a goroutine that deletes expired rentals every minute.
func StartExpiredRentalCleanup(db *sql.DB) {
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        for range ticker.C {
            n, err := DeleteExpiredRentals(db)
            if err != nil {
                log.Printf("Error deleting expired rentals: %v", err)
            } else if n > 0 {
                log.Printf("Cleaned up %d expired rentals", n)
            }
        }
    }()
}
