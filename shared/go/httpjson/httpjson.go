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
