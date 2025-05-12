package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/smeetnagda/vmshare/internal/multipass"
)

// CreateRentalRequest represents the JSON payload to create a VM rental.
type CreateRentalRequest struct {
	UserID   int    `json:"user_id"`
	SSHKey   string `json:"ssh_key"`   // Public SSH key for cloud-init
	Duration int    `json:"duration"`  // Minutes
}

// CreateRentalResponse is returned after a rental is created.
type CreateRentalResponse struct {
	VMName    string    `json:"vm_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

// HandleCreateRental handles POST /rentals
func HandleCreateRental(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Decode request
		var req CreateRentalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate unique VM name
		vmName := fmt.Sprintf("rental-%d-%d", req.UserID, time.Now().Unix())
		expiresAt := time.Now().Add(time.Duration(req.Duration) * time.Minute)

		// Insert rental record (agentID=0 placeholder)
		if _, err := db.Exec(
			"INSERT INTO rentals(vm_name,user_id,agent_id,expires_at) VALUES(?,?,?,?)",
			vmName, req.UserID, 0, expiresAt,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Launch VM via Multipass CLI
		if err := multipass.StartVM(vmName, req.SSHKey, time.Duration(req.Duration)*time.Minute); err != nil {
			http.Error(w, fmt.Sprintf("failed to start VM: %v", err), http.StatusInternalServerError)
			return
		}

		// Respond with VM details
		resp := CreateRentalResponse{
			VMName:    vmName,
			ExpiresAt: expiresAt,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleDeleteRental handles DELETE /rentals/{vmName}
func HandleDeleteRental(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract vmName from URL path
		vmName := path.Base(r.URL.Path)
		if vmName == "" {
			http.Error(w, "missing vmName", http.StatusBadRequest)
			return
		}

		// Delete VM via Multipass CLI
		if err := multipass.DeleteVM(vmName); err != nil {
			http.Error(w, fmt.Sprintf("failed to delete VM: %v", err), http.StatusInternalServerError)
			return
		}

		// Remove rental record from DB
		if _, err := db.Exec("DELETE FROM rentals WHERE vm_name = ?", vmName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
