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
	mux.HandleFunc("/api/v1/ingest", h.ingest)
	mux.HandleFunc("/api/v1/agents", h.agents)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "gateway"})
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpjson.MethodNotAllowed(w)
		return
	}
	var req api.IngestRequest
	if err := httpjson.Decode(r, &req); err != nil {
		httpjson.BadRequest(w, err)
		return
	}
	if req.Type == "" {
		httpjson.BadRequest(w, apiError("type is required"))
		return
	}
	if h.store != nil {
		switch req.Type {
		case "inventory":
			_ = h.store.UpsertInventory(context.Background(), api.InventoryRecord{
				AgentID: req.AgentID, Hostname: req.Hostname, OSFamily: req.OSFamily,
				OSVersion: req.OSVersion, Architecture: req.Architecture, Software: req.Software, Attributes: req.Payload,
			})
		case "compliance":
			_ = h.store.SaveComplianceReport(context.Background(), api.ComplianceReport{
				AgentID: req.AgentID, Status: req.Status, Findings: req.Findings,
			})
		}
	}
	httpjson.WriteJSON(w, http.StatusAccepted, api.AcceptedResponse{Status: "accepted"})
}

func (h *Handler) agents(w http.ResponseWriter, _ *http.Request) {
	if h.store != nil {
		agents, err := h.store.ListAgents(context.Background())
		if err == nil {
			httpjson.WriteJSON(w, http.StatusOK, agents)
			return
		}
	}
	httpjson.WriteJSON(w, http.StatusOK, []api.AgentSummary{
		{
			ID: "dev-agent-001", TenantID: "dev-tenant-001", Hostname: "local-agent",
			OSFamily: "darwin", OSVersion: "dev-local", Architecture: "arm64", Status: "online",
		},
	})
}

type apiError string

func (e apiError) Error() string {
	return string(e)
}
