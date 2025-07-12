package server

import (
	"database/sql"
	"time"
)

// --- User Model & Helpers ---

// User represents a platform user who can rent VMs.
type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`       // stored as a bcrypt hash
	SSHKey    string    `json:"ssh_key"` // public key for VM access
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser inserts a new user and returns its ID.
func CreateUser(db *sql.DB, email, hashedPassword, sshKey string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO users (email, password, ssh_key) VALUES (?, ?, ?)`,
		email, hashedPassword, sshKey,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetUserByEmail looks up a user by email. Returns nil, nil if not found.
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, email, password, ssh_key, created_at
		   FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.Password, &u.SSHKey, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// --- Agent Model & Helpers ---

// Agent represents a host/agent that runs VMs.
type Agent struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	LastSeen    time.Time `json:"last_seen"`
	Capacity    int       `json:"capacity"`
	BaseSSHPort int       `json:"base_ssh_port"`
}

// UpsertAgent updates an existing agent or inserts a new one, returning its ID.
func UpsertAgent(db *sql.DB, name string, capacity, baseSSHPort int, lastSeen time.Time) (int64, error) {
	// Try to update first
	_, err := db.Exec(
		`UPDATE agents
		   SET last_seen = ?, capacity = ?, base_ssh_port = ?
		 WHERE name = ?`,
		lastSeen, capacity, baseSSHPort, name,
	)
	if err != nil {
		return 0, err
	}
	// Insert if not already present
	res, err := db.Exec(
		`INSERT OR IGNORE INTO agents
		   (name, last_seen, capacity, base_ssh_port)
		 VALUES (?, ?, ?, ?)`,
		name, lastSeen, capacity, baseSSHPort,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// --- Rental Model & Helpers ---

// Rental represents a VM rental reservation.
type Rental struct {
	ID        int            `json:"id"`
	VMName    string         `json:"vm_name"`
	UserID    int            `json:"user_id"`
	AgentID   int            `json:"agent_id"`
	IPAddress sql.NullString `json:"ip_address"`
	ExpiresAt time.Time      `json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
}

// CreateRental reserves a VM slot and returns the new row ID.
func CreateRental(db *sql.DB, vmName string, userID, agentID int, expiresAt time.Time) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO rentals
		   (vm_name, user_id, agent_id, expires_at)
		 VALUES (?, ?, ?, ?)`,
		vmName, userID, agentID, expiresAt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateRentalIP sets the VM's IP address once known.
func UpdateRentalIP(db *sql.DB, vmName, ip string) error {
	_, err := db.Exec(
		`UPDATE rentals SET ip_address = ? WHERE vm_name = ?`,
		ip, vmName,
	)
	return err
}

// DeleteExpiredRentals removes any rentals past their expiration.
// It returns the number of rows deleted.
func DeleteExpiredRentals(db *sql.DB) (int64, error) {
	res, err := db.Exec(`DELETE FROM rentals WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
