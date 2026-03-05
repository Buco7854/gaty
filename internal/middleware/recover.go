package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// JSONRecoverer replaces chi's plain-text Recoverer with one that always
// returns RFC-7807 / Huma-compatible JSON so the frontend always gets JSON.
func JSONRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "stack", string(debug.Stack()))
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)
				body, _ := json.Marshal(map[string]any{
					"title":  "Internal Server Error",
					"status": 500,
				})
				_, _ = w.Write(body)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
