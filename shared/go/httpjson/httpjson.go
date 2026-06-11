package httpjson

import (
	"encoding/json"
	"net/http"

	"example.com/rmm-shared/api"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func Decode(r *http.Request, out any) error {
	return json.NewDecoder(r.Body).Decode(out)
}

func MethodNotAllowed(w http.ResponseWriter) {
	WriteJSON(w, http.StatusMethodNotAllowed, api.ErrorResponse{Error: "method not allowed"})
}

func BadRequest(w http.ResponseWriter, err error) {
	WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
}

// WithCORS wraps a handler and adds permissive CORS headers for local development.
// The dashboard runs on localhost:5173 and needs to reach the backend services.
func WithCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}
