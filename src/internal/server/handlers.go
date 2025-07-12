package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"strconv"

	"github.com/smeetnagda/vmshare/internal/multipass"

)

// CreateRentalRequest defines the payload for creating a rental.
type CreateRentalRequest struct {
	UserID   int    `json:"user_id"`
	SSHKey   string `json:"ssh_key"`
	Duration int    `json:"duration"` // in minutes
}

// CreateRentalResponse returns the VM name and expiration.
type CreateRentalResponse struct {
	VMName    string    `json:"vm_name"`
	ExpiresAt time.Time `json:"expires_at"`
}
type ExtendRentalRequest struct {
	    Duration int `json:"duration"` // minutes to add
}
type ExtendRentalResponse struct {
    VMName    string    `json:"vm_name"`
    ExpiresAt time.Time `json:"expires_at"`
}


// RentalsHandler dispatches GET->List, POST->Create
func RentalsHandler(db *sql.DB) http.HandlerFunc {
    list := HandleListRentals(db)
    create := HandleCreateRental(db)
    return func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            list(w, r)
        case http.MethodPost:
            create(w, r)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    }
}

func HandleListRentals(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        rows, err := db.Query(`
            SELECT id, vm_name, user_id, agent_id, ip_address, expires_at, created_at
            FROM rentals
        `)
        if err != nil {
            http.Error(w, fmt.Sprintf("failed to query rentals: %v", err), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var list []Rental
        for rows.Next() {
            var rec Rental
            if err := rows.Scan(
                &rec.ID,
                &rec.VMName,
                &rec.UserID,
                &rec.AgentID,
                &rec.IPAddress,
                &rec.ExpiresAt,
                &rec.CreatedAt,
            ); err != nil {
                http.Error(w, fmt.Sprintf("failed to scan rental: %v", err), http.StatusInternalServerError)
                return
            }
            list = append(list, rec)
        }
        if err := rows.Err(); err != nil {
            http.Error(w, fmt.Sprintf("error iterating rentals: %v", err), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(list)
    }
}
// HandleCreateRental handles POST /rentals to create a new VM rental.
func HandleCreateRental(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CreateRentalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
			return
		}

		// Generate a unique VM name
		vmName := fmt.Sprintf("rental-%d-%d", req.UserID, time.Now().Unix())
		expiresAt := time.Now().Add(time.Duration(req.Duration) * time.Minute)

		// Persist the rental
		if _, err := db.Exec(
			`INSERT INTO rentals(vm_name, user_id, ssh_key, agent_id, expires_at)
			 VALUES (?, ?, ?, ?, ?)`,
			vmName, req.UserID, req.SSHKey, 0, expiresAt,
		); err != nil {
			http.Error(w, fmt.Sprintf("failed to create rental: %v", err), http.StatusInternalServerError)
			return
		}

		resp := CreateRentalResponse{VMName: vmName, ExpiresAt: expiresAt}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleDeleteRental handles DELETE /rentals/{vmName} to tear down and remove a rental.
func HandleDeleteRental(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract vmName from URL path
		vmName := strings.TrimPrefix(r.URL.Path, "/rentals/")
		if vmName == "" {
			http.NotFound(w, r)
			return
		}

		// Attempt to delete the VM
		if err := multipass.DeleteVM(vmName); err != nil {
			http.Error(w, fmt.Sprintf("failed to delete VM: %v", err), http.StatusInternalServerError)
			return
		}

		// Remove from database
		if _, err := db.Exec(`DELETE FROM rentals WHERE vm_name = ?`, vmName); err != nil {
			http.Error(w, fmt.Sprintf("failed to remove rental: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
func HandleExtendRental(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPatch {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        // expect path like "/rentals/{vmName}/extend"
        parts := strings.Split(r.URL.Path, "/")
        if len(parts) != 4 || parts[3] != "extend" {
            http.NotFound(w, r)
            return
        }
        vmName := parts[2]

        var req ExtendRentalRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
            return
        }
        if req.Duration <= 0 {
            http.Error(w, "duration must be > 0", http.StatusBadRequest)
            return
        }

        // bump expires_at
        res, err := db.Exec(
            `UPDATE rentals 
             SET expires_at = datetime(expires_at, '+' || ? || ' minutes')
             WHERE vm_name = ?`,
            strconv.Itoa(req.Duration), vmName,
        )
        if err != nil {
            http.Error(w, fmt.Sprintf("failed to extend rental: %v", err), http.StatusInternalServerError)
            return
        }
        n, _ := res.RowsAffected()
        if n == 0 {
            http.Error(w, "rental not found", http.StatusNotFound)
            return
        }

        // fetch new expiration
        var newExpiry time.Time
        if err := db.QueryRow(
            `SELECT expires_at FROM rentals WHERE vm_name = ?`, vmName,
        ).Scan(&newExpiry); err != nil {
            http.Error(w, fmt.Sprintf("could not fetch new expiry: %v", err), http.StatusInternalServerError)
            return
        }

        resp := ExtendRentalResponse{VMName: vmName, ExpiresAt: newExpiry}
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }
}