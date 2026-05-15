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
	mux.HandleFunc("/api/v1/compliance/reports", h.reports)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "compliance"})
}

func (h *Handler) reports(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.store != nil {
			reports, err := h.store.ListComplianceReports(context.Background())
			if err == nil {
				httpjson.WriteJSON(w, http.StatusOK, reports)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusOK, []api.ComplianceReport{
			{
				AgentID: "dev-agent-001",
				Status:  "needs_review",
				Findings: []api.ComplianceFinding{
					{Category: "patch", ResourceID: "KB5031538", Status: "blocked_by_prerequisite", Reason: ".NET disabled"},
				},
			},
		})
	case http.MethodPost:
		var req api.ComplianceReport
		if err := httpjson.Decode(r, &req); err != nil {
			httpjson.BadRequest(w, err)
			return
		}
		if req.AgentID == "" || req.Status == "" {
			httpjson.BadRequest(w, apiError("agent_id and status are required"))
			return
		}
		if h.store != nil {
			if err := h.store.SaveComplianceReport(context.Background(), req); err != nil {
				httpjson.BadRequest(w, err)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusAccepted, api.AcceptedResponse{Status: "stored"})
	default:
		httpjson.MethodNotAllowed(w)
	}
}

type apiError string

func (e apiError) Error() string {
	return string(e)
}
