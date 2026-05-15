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
	mux.HandleFunc("/api/v1/inventory", h.inventory)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "inventory"})
}

func (h *Handler) inventory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.store != nil {
			records, err := h.store.ListInventory(context.Background())
			if err == nil {
				httpjson.WriteJSON(w, http.StatusOK, records)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusOK, []api.InventoryRecord{
			{
				AgentID: "dev-agent-001", Hostname: "local-agent", OSFamily: "darwin",
				OSVersion: "dev-local", Architecture: "arm64",
			},
		})
	case http.MethodPost:
		var req api.InventoryRecord
		if err := httpjson.Decode(r, &req); err != nil {
			httpjson.BadRequest(w, err)
			return
		}
		if req.AgentID == "" || req.Hostname == "" {
			httpjson.BadRequest(w, apiError("agent_id and hostname are required"))
			return
		}
		if h.store != nil {
			if err := h.store.UpsertInventory(context.Background(), req); err != nil {
				httpjson.BadRequest(w, err)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusAccepted, api.InventoryIngestResponse{Status: "queued"})
	default:
		httpjson.MethodNotAllowed(w)
	}
}

type apiError string

func (e apiError) Error() string {
	return string(e)
}
