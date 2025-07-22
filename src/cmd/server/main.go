package main
import (
    "log"
    "net/http"
    "os"
    "path"
    "github.com/gorilla/handlers"
    "github.com/smeetnagda/vmshare/internal/server"
)

func main() {
    dbPath := "data/vmrental.db"
    db, err := server.NewDB(dbPath)
    if err != nil {
        log.Fatalf("Failed to open database %q: %v", dbPath, err)
    }
    defer db.Close()
    log.Printf("✅ Database ready: %s", dbPath)
    server.StartExpiredRentalCleanup(db)

    mux := http.NewServeMux()
    mux.HandleFunc("/rentals", server.RentalsHandler(db))
    mux.HandleFunc("/rentals/", func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.Method == http.MethodDelete:
            server.HandleDeleteRental(db)(w, r)
        case r.Method == http.MethodPatch && path.Base(r.URL.Path) == "extend":
            server.HandleExtendRental(db)(w, r)
        default:
            http.NotFound(w, r)
        }
    })
    mux.HandleFunc("/signup", server.HandleSignup(db))
    mux.HandleFunc("/login",  server.HandleLogin(db))
    mux.HandleFunc("/me",     server.HandleGetCurrentUser(db))
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.Write([]byte("OK"))
    })
    mux.HandleFunc("/logout", server.LogoutHandler())
    // Configure CORS:
    corsHandler := handlers.CORS(
        handlers.AllowedOrigins([]string{"http://localhost:3000"}),         // your React app
        handlers.AllowedMethods([]string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}),
        handlers.AllowedHeaders([]string{"Content-Type", "Authorization","Cookie"}),
        handlers.AllowCredentials(),
    )(mux)

    addr := os.Getenv("HTTP_ADDR")
    if addr == "" {
        addr = ":8080"
    }
    log.Printf("🚀 Coordinator listening on %s …", addr)
    if err := http.ListenAndServe(addr, corsHandler); err != nil {
        log.Fatalf("HTTP server error: %v", err)
    }
}
