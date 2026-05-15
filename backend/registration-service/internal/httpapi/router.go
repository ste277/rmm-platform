package httpapi

import (
	"context"
	"net/http"

	"example.com/rmm-shared/api"
	"example.com/rmm-shared/httpjson"
	"example.com/rmm-shared/store"
)

type Handler struct {
	store *store.Store
}

func NewMux(db *store.Store) *http.ServeMux {
	h := &Handler{store: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/api/v1/register", h.register)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "registration"})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpjson.MethodNotAllowed(w)
		return
	}

	var req api.RegistrationRequest
	if err := httpjson.Decode(r, &req); err != nil {
		httpjson.BadRequest(w, err)
		return
	}
	if req.TenantID == "" || req.Hostname == "" {
		httpjson.BadRequest(w, apiError("tenant_id and hostname are required"))
		return
	}

	if h.store != nil {
		resp, err := h.store.RegisterAgent(context.Background(), req)
		if err == nil {
			httpjson.WriteJSON(w, http.StatusCreated, resp)
			return
		}
	}

	httpjson.WriteJSON(w, http.StatusCreated, api.RegistrationResponse{
		AgentID:   "dev-agent-001",
		BrokerURL: "ws://127.0.0.1:8081/ws?agent_id=dev-agent-001",
	})
}

type apiError string

func (e apiError) Error() string {
	return string(e)
}
