package server

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
	"log"

    "golang.org/x/crypto/bcrypt"
    "github.com/gorilla/sessions"
)

type SignupRequest struct {
    Email     string `json:"name"`
    Password string `json:"password"`
	SSHKey   string `json:"ssh_key"`
}

type LoginRequest struct {
    Email     string `json:"name"`
    Password string `json:"password"`
}

// Session store (you can move this somewhere more central)
var Store = sessions.NewCookieStore([]byte("Vn9gL5xE2qZ7TJk81rMfA0bYD6hW3uCpNeQsR4oXvltUGaK9"))

func HandleSignup(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Printf("[SIGNUP] %s %s", r.Method, r.URL.Path)
        var req SignupRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            log.Printf("[SIGNUP] decode error: %v", err)
            http.Error(w, "invalid request", http.StatusBadRequest)
            return
        }
        // log the payload (never log raw passwords in prod!)
        log.Printf(`[SIGNUP] payload: email=%q, password=%q, sshKey=%q`,
            req.Email, req.Password, req.SSHKey)

        // hash password
        hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
        if err != nil {
            log.Printf("[SIGNUP] bcrypt error: %v", err)
            http.Error(w, "server error", http.StatusInternalServerError)
            return
        }

        // now insert
        id, err := CreateUser(db, req.Email, string(hashed), req.SSHKey)
        if err != nil {
            log.Printf("[SIGNUP] CreateUser error: %v", err)
            http.Error(w, "could not create user", http.StatusInternalServerError)
            return
        }
        log.Printf("[SIGNUP] user created: id=%d, email=%s", id, req.Email)

        w.WriteHeader(http.StatusCreated)
        fmt.Fprint(w, id)
    }
}
func HandleGetCurrentUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	  // grab the session
	  sess, _ := Store.Get(r, "vmshare-session")
	  uidRaw, ok := sess.Values["user_id"]
	  if !ok {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	  }
	  userID, ok := uidRaw.(int)
	  if !ok {
		http.Error(w, "invalid session data", http.StatusUnauthorized)
		return
	  }
  
	  // load user from DB
	  var u struct {
		ID       int    `json:"id"`
		Email    string `json:"email"`
		SSHPublicKey string `json:"ssh_key"`
	  }
	  err := db.QueryRow(
		`SELECT id,email,ssh_key FROM users WHERE id = ?`, userID,
	  ).Scan(&u.ID, &u.Email, &u.SSHPublicKey)
	  if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	  }
  
	  w.Header().Set("Content‑Type", "application/json")
	  json.NewEncoder(w).Encode(u)
	}
  }
// HandleLogin POST /login
func HandleLogin(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Println("[LOGIN] new request")                              // 1
        if r.Method != http.MethodPost {
            log.Println("[LOGIN] wrong method:", r.Method)             // 2
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req LoginRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            log.Printf("[LOGIN] decode error: %v\n", err)               // 3
            http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
            return
        }
        defer r.Body.Close()
        log.Printf("[LOGIN] payload: email=%q, password=%q\n", req.Email, req.Password) // 4 (you may want to omit logging the raw password in prod)

        var id int
        var hash string
        err := db.QueryRow(
            `SELECT id, password FROM users WHERE email = ?`, req.Email,
        ).Scan(&id, &hash)
        if err == sql.ErrNoRows {
            log.Printf("[LOGIN] user not found: %q\n", req.Email)        // 5
            http.Error(w, "invalid credentials", http.StatusUnauthorized)
            return
        } else if err != nil {
            log.Printf("[LOGIN] db error: %v\n", err)                   // 6
            http.Error(w, fmt.Sprintf("db error: %v", err), http.StatusInternalServerError)
            return
        }
        log.Printf("[LOGIN] fetched id=%d, hash=%q\n", id, hash)         // 7

        if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
            log.Printf("[LOGIN] bcrypt mismatch: %v\n", err)             // 8
            http.Error(w, "invalid credentials", http.StatusUnauthorized)
            return
        }
		sess, _ := Store.Get(r, "vmshare-session")
		sess.Values["user_id"] = id
		sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 24,        // 1 day
		HttpOnly: true,             // not available to JS
		Secure:   true,             // only over HTTPS (in prod)
		SameSite: http.SameSiteLaxMode,
		}

		// write the cookie back to the response
		if err := sess.Save(r, w); err != nil {
		http.Error(w, "could not save session", http.StatusInternalServerError)
		return
		}

        log.Printf("[LOGIN] success for user id=%d\n", id)               // 9
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    }
}
// LogoutHandler clears the session and returns 204 No Content.
func LogoutHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // grab the session
        sess, _ := Store.Get(r, "vmshare-session")
        // wipe its contents
        sess.Options.MaxAge = -1
        sess.Save(r, w)
        w.WriteHeader(http.StatusNoContent)
    }
}


