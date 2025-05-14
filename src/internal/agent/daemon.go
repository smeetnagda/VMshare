// internal/agent/daemon.go
package agent

import (
    "database/sql"
    "fmt"
    "time"
    "strings"

    _ "github.com/mattn/go-sqlite3"
    "github.com/smeetnagda/vmshare/internal/multipass"
)
func execWithRetry(db *sql.DB, query string, args ...interface{}) error {
    for i := 0; i < 5; i++ {
        if _, err := db.Exec(query, args...); err != nil {
            if strings.Contains(err.Error(), "database is locked") {
                time.Sleep(200 * time.Millisecond)
                continue
            }
            return err
        }
        return nil
    }
    return fmt.Errorf("exec retry failed: %s %v", query, args)
}
// Run continuously polls the rentals table, starts pending VMs, updates their IPs,
// and tears down any VMs whose leases have expired.
func Run(dbPath string, agentID int) error {
    db, err := sql.Open("sqlite3",
    fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath),
    )
    if err != nil {
        return fmt.Errorf("open db: %v", err)
    }

    for {
        now := time.Now()

        // --- A) Launch pending VMs ---
        rows, err := db.Query(
            `SELECT vm_name, ssh_key, expires_at
             FROM rentals
             WHERE ip_address IS NULL AND expires_at > ?`,
            now,
        )
        if err != nil {
            fmt.Printf("query pending rentals: %v\n", err)
        } else {
            for rows.Next() {
                var vmName, sshKey string
                var expiresAt time.Time
                if err := rows.Scan(&vmName, &sshKey, &expiresAt); err != nil {
                    fmt.Printf("scan rental row: %v\n", err)
                    continue
                }
                go func(vmName, sshKey string, expiresAt time.Time) {
                    duration := time.Until(expiresAt)
                    if err := multipass.StartVM(vmName, sshKey, duration); err != nil {
                        fmt.Printf("failed to start VM %s: %v\n", vmName, err)
                        return
                    }
                    // Fetch IP and persist it
                    ip, err := multipass.FetchIP(vmName)
                    if err != nil {
                        fmt.Printf("failed to fetch IP for %s: %v\n", vmName, err)
                    } else {
                        if _, err := db.Exec(
                            `UPDATE rentals SET ip_address = ?, agent_id = ? WHERE vm_name = ?`,
                            ip, agentID, vmName,
                        ); err != nil {
                            fmt.Printf("failed to update rental %s: %v\n", vmName, err)
                        }
                    }
                }(vmName, sshKey, expiresAt)
            }
            rows.Close()
        }

        // --- B) Tear down expired VMs ---
        rows2, err := db.Query(
            `SELECT vm_name
             FROM rentals
             WHERE ip_address IS NOT NULL AND expires_at <= ?`,
            now,
        )
        if err != nil {
            fmt.Printf("query expired rentals: %v\n", err)
        } else {
            for rows2.Next() {
                var vmName string
                if err := rows2.Scan(&vmName); err != nil {
                    fmt.Printf("scan expired row: %v\n", err)
                    continue
                }
                if err := multipass.DeleteVM(vmName); err != nil {
                    // if multipass says “does not exist”, ignore it
                    if !strings.Contains(err.Error(), "does not exist") {
                        fmt.Printf("deleteVM %s error: %v\n", vmName, err)
                    }
                }
                if  err := execWithRetry(db,
                    `DELETE FROM rentals WHERE vm_name = ?`,
                    vmName,
                ); err != nil {
                    fmt.Printf("failed to remove rental %s: %v\n", vmName, err)
                }
            }
            rows2.Close()
        }

        time.Sleep(10 * time.Second)
    }
}
