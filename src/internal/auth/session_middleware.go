package auth

import (
    "net/http"
    "github.com/gorilla/sessions"
)

func SessionMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, _ := Store.Get(r, "vmshare-session")
			uid, ok := sess.Values["user_id"].(int)
			if !ok {
			  http.Error(w, "unauthorized", http.StatusUnauthorized)
			  return
			}
			// stash it in context for downstream handlers
			ctx := context.WithValue(r.Context(), "user_id", uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		  })
    }
}
