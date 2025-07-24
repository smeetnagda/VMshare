package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"strconv"
    "context"
	"github.com/smeetnagda/vmshare/internal/multipass"

)
type ctxKey string

const userIDKey ctxKey = "user_id"

// CreateRentalRequest defines the payload for creating a rental.
type CreateRentalRequest struct {
    Duration int    `json:"duration"`
    SSHKey   string `json:"ssh_key"`
    CPUs     int    `json:"cpus"`
    Memory   int    `json:"memory"`
    Disk     int    `json:"disk"`
}

type RentalResponse struct {
    VMName    string    `json:"vm_name"`
    ExpiresAt time.Time `json:"expires_at"`
    CPUs      int       `json:"cpus"`
    Memory    int       `json:"memory"`
    Disk      int       `json:"disk"`
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
        // 1) grab session
        sess, err := Store.Get(r, "vmshare-session")
        if err != nil {
            http.Error(w, "session error", http.StatusInternalServerError)
            return
        }

        // 2) pull out user_id
        rawUID, ok := sess.Values["user_id"]
        if !ok {
            http.Error(w, "not authenticated", http.StatusUnauthorized)
            return
        }
        uid, ok := rawUID.(int)
        if !ok {
            http.Error(w, "invalid session", http.StatusUnauthorized)
            return
        }

        // 3) stash into context
        ctx := context.WithValue(r.Context(), userIDKey, uid)
        r = r.WithContext(ctx)

        // 4) dispatch to list or create
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
        rows, err := db.Query(`
            SELECT id, vm_name, user_id, agent_id, ip_address,
                   expires_at, cpus, memory, disk, created_at
              FROM rentals
             WHERE user_id = ?`, /* or no filter if global */)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var out []RentalResponse
        for rows.Next() {
            var rr RentalResponse
            if err := rows.Scan(
                &rr.VMName,
                &rr.ExpiresAt,
                &rr.CPUs, &rr.Memory, &rr.Disk,
            ); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            out = append(out, rr)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(out)
    }
}

// HandleCreateRental handles POST /rentals to create a new VM rental.
func HandleCreateRental(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req CreateRentalRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "invalid request", http.StatusBadRequest)
            return
        }
        uid := r.Context().Value(userIDKey).(int)

        vmName := fmt.Sprintf("rental-%d-%d", uid, time.Now().Unix())
        expiresAt := time.Now().Add(time.Duration(req.Duration) * time.Minute)

        // Persist cpus, memory, disk too:
        if _, err := db.Exec(
            `INSERT INTO rentals
              (vm_name, user_id, ssh_key, agent_id, expires_at, cpus, memory, disk)
             VALUES (?, ?, ?, 0, ?, ?, ?, ?)`,
            vmName, uid, req.SSHKey, expiresAt,
            req.CPUs, req.Memory, req.Disk,
        ); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        resp := RentalResponse{
            VMName:    vmName,
            ExpiresAt: expiresAt,
            CPUs:      req.CPUs,
            Memory:    req.Memory,
            Disk:      req.Disk,
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
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