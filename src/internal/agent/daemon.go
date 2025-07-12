package agent

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/smeetnagda/vmshare/internal/system"
)

// Run starts the agent daemon loop, polling rentals and managing VMs.
func Run(dbPath string, agentID int) error {
	// open in WAL mode so our long-running agent can update safely
	db, err := sql.Open(
		"sqlite3",
		fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath),
	)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	for {
		now := time.Now()

		// ─── Creation pass: launch any rental that has no IP yet ───
		rows, err := db.Query(`
			SELECT vm_name, ssh_key, expires_at
			FROM rentals
			WHERE ip_address IS NULL
			  AND expires_at > ?`,
			now,
		)
		if err != nil {
			fmt.Printf("query pending rentals: %v\n", err)
		} else {
			for rows.Next() {
				var vmName, sshKey string
				var expiresAt time.Time
				if err := rows.Scan(&vmName, &sshKey, &expiresAt); err != nil {
                    fmt.Printf("scan row error: %v\n", err)
                    continue
                }
            

				duration := time.Until(expiresAt)
				hostPort, err := system.StartVM(vmName, sshKey, duration)
                if err != nil {
					fmt.Printf("startVM %s error: %v\n", vmName, err)
					continue
				}
				// we always forward guest:22 → localhost:2222
				addr := fmt.Sprintf("127.0.0.1:%d", hostPort)

				// wait for SSH socket to become ready
				for i := 0; i < 15; i++ {
					conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
					if err == nil {
						conn.Close()
						break
					}
					time.Sleep(2 * time.Second)
				}

				// persist that endpoint into the DB
				if _, err := db.Exec(
					`UPDATE rentals SET ip_address = ?, agent_id = ? WHERE vm_name = ?`,
					addr, agentID, vmName,
				); err != nil {
					fmt.Printf("update rental %s error: %v\n", vmName, err)
				} else {
					fmt.Printf("✅ VM %q ready; SSH at: ssh -p %d ubuntu@127.0.0.1\n", vmName, hostPort)
				}
			}
			rows.Close()
		}

		// ─── Deletion pass: remove any expired rows ───
		// (the QEMU process itself already self-kills after duration)
		rows2, err2 := db.Query(`
			SELECT vm_name
			FROM rentals
			WHERE expires_at <= ?`,
			now,
		)
		if err2 != nil {
			fmt.Printf("query expired rentals: %v\n", err2)
		} else {
			for rows2.Next() {
				var vmName string
				rows2.Scan(&vmName)

				// just delete the database record
				if _, err := db.Exec(
					`DELETE FROM rentals WHERE vm_name = ?`, vmName,
				); err != nil {
					fmt.Printf("delete rental %s error: %v\n", vmName, err)
				} else {
					fmt.Printf("🗑️ Cleaned up rental %q\n", vmName)
				}
			}
			rows2.Close()
		}

		time.Sleep(10 * time.Second)
	}
}
