package server

import (
    "database/sql"
    "time"
)

// User represents a user account.
type User struct {
    ID             int
    Name           string
    HashedPassword string
}

// CreateUser inserts a new user and returns its ID.
func CreateUser(db *sql.DB, name, hashedPassword string) (int64, error) {
    res, err := db.Exec(
        `INSERT INTO users (name, hashed_password) VALUES (?, ?)`,
        name, hashedPassword,
    )
    if err != nil {
        return 0, err
    }
    return res.LastInsertId()
}

// Agent represents a host agent.
type Agent struct {
    ID          int
    Name        string
    LastSeen    time.Time
    Capacity    int
    BaseSSHPort int
}

// UpsertAgent inserts or updates an agent’s last_seen and capacity.
func UpsertAgent(db *sql.DB, name string, capacity, baseSSHPort int, lastSeen time.Time) (int64, error) {
    // Try update first
    if _, err := db.Exec(
        `UPDATE agents SET last_seen=?, capacity=?, base_ssh_port=? WHERE name=?`,
        lastSeen, capacity, baseSSHPort, name,
    ); err != nil {
        return 0, err
    }
    // Then insert if no rows updated
    res, err := db.Exec(
        `INSERT OR IGNORE INTO agents (name, last_seen, capacity, base_ssh_port) VALUES (?, ?, ?, ?)`,
        name, lastSeen, capacity, baseSSHPort,
    )
    if err != nil {
        return 0, err
    }
    return res.LastInsertId()
}

// Rental represents a VM rental.
type Rental struct {
    ID        int
    VMName    string
    UserID    int
    AgentID   int
    IPAddress sql.NullString
    ExpiresAt time.Time
    CreatedAt time.Time
}

// CreateRental reserves a VM slot.
func CreateRental(db *sql.DB, vmName string, userID, agentID int, expiresAt time.Time) (int64, error) {
    res, err := db.Exec(
        `INSERT INTO rentals (vm_name, user_id, agent_id, expires_at)
         VALUES (?, ?, ?, ?)`,
        vmName, userID, agentID, expiresAt,
    )
    if err != nil {
        return 0, err
    }
    return res.LastInsertId()
}

// UpdateRentalIP sets the IP address once the VM is up.
func UpdateRentalIP(db *sql.DB, vmName, ip string) error {
    _, err := db.Exec(
        `UPDATE rentals SET ip_address=? WHERE vm_name=?`,
        ip, vmName,
    )
    return err
}

// DeleteExpiredRentals removes any rentals past their expires_at.
func DeleteExpiredRentals(db *sql.DB) (int64, error) {
    res, err := db.Exec(`DELETE FROM rentals WHERE expires_at < ?`, time.Now())
    if err != nil {
        return 0, err
    }
    return res.RowsAffected()
}

