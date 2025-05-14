package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
