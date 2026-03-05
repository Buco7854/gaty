package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/danielgtaylor/huma/v2"
)

// JSONRecoverer replaces chi's plain-text Recoverer. It returns panics as
// huma.ErrorModel (RFC 9457 / application/problem+json) so the response
// schema matches every other error in the OpenAPI spec.
func JSONRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "stack", string(debug.Stack()))
				model := huma.NewError(http.StatusInternalServerError, "Internal Server Error")
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)
				body, _ := json.Marshal(model)
				_, _ = w.Write(body)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
