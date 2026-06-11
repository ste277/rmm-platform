package httpapi

import (
	"context"
	"net/http"
	"strings"

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

	// Public
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/api/v1/ingest", h.ingest)

	// Agents
	mux.HandleFunc("/api/v1/agents", h.withAuth(h.agents))

	// Metrics
	mux.HandleFunc("/api/v1/metrics/", h.withAuth(h.metrics))

	// API Keys
	mux.HandleFunc("/api/v1/keys", h.withAuth(h.apiKeys))

	// Scripts
	mux.HandleFunc("/api/v1/scripts", h.withAuth(h.scripts))
	mux.HandleFunc("/api/v1/scripts/", h.withAuth(h.scriptByID))

	// Alert rules
	mux.HandleFunc("/api/v1/alert-rules", h.withAuth(h.alertRules))
	mux.HandleFunc("/api/v1/alert-rules/", h.withAuth(h.alertRuleByID))

	// Alert events
	mux.HandleFunc("/api/v1/alert-events", h.withAuth(h.alertEvents))

	return mux
}

func (h *Handler) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.store != nil {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				if _, err := h.store.RequireAPIKey(r.Context(), authHeader); err != nil {
					httpjson.WriteJSON(w, http.StatusUnauthorized,
						api.ErrorResponse{Error: "invalid or missing API key"})
					return
				}
			}
		}
		next(w, r)
	}
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
				OSVersion: req.OSVersion, Architecture: req.Architecture,
				Software: req.Software, Attributes: req.Payload,
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
			OSFamily: "darwin", OSVersion: "macOS 14.0", Architecture: "arm64", Status: "online",
		},
	})
}

func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpjson.MethodNotAllowed(w)
		return
	}
	agentID := strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/")
	if agentID == "" {
		httpjson.BadRequest(w, apiError("agent_id is required in path"))
		return
	}
	if h.store != nil {
		samples, err := h.store.GetRecentMetrics(context.Background(), agentID, 200)
		if err == nil {
			httpjson.WriteJSON(w, http.StatusOK, api.AgentMetrics{AgentID: agentID, Points: samples})
			return
		}
	}
	httpjson.WriteJSON(w, http.StatusOK, api.AgentMetrics{AgentID: agentID, Points: nil})
}

func (h *Handler) apiKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpjson.MethodNotAllowed(w)
		return
	}
	var req api.CreateAPIKeyRequest
	if err := httpjson.Decode(r, &req); err != nil {
		httpjson.BadRequest(w, err)
		return
	}
	if req.TenantID == "" || req.Name == "" {
		httpjson.BadRequest(w, apiError("tenant_id and name are required"))
		return
	}
	if h.store == nil {
		httpjson.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Error: "database not available"})
		return
	}
	resp, err := h.store.CreateAPIKey(context.Background(), req.TenantID, req.Name)
	if err != nil {
		httpjson.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, resp)
}

// ── Scripts ──────────────────────────────────────────────────────────────────

func (h *Handler) scripts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			tenantID = "dev-tenant-001"
		}
		if h.store != nil {
			scripts, err := h.store.ListScripts(context.Background(), tenantID)
			if err == nil {
				httpjson.WriteJSON(w, http.StatusOK, scripts)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusOK, []store.Script{})
	case http.MethodPost:
		var req struct {
			TenantID    string `json:"tenant_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Body        string `json:"body"`
		}
		if err := httpjson.Decode(r, &req); err != nil {
			httpjson.BadRequest(w, err)
			return
		}
		if req.Name == "" || req.Body == "" {
			httpjson.BadRequest(w, apiError("name and body are required"))
			return
		}
		if req.TenantID == "" {
			req.TenantID = "dev-tenant-001"
		}
		if h.store == nil {
			httpjson.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Error: "no database"})
			return
		}
		sc, err := h.store.CreateScript(context.Background(), req.TenantID, req.Name, req.Description, req.Body)
		if err != nil {
			httpjson.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}
		httpjson.WriteJSON(w, http.StatusCreated, sc)
	default:
		httpjson.MethodNotAllowed(w)
	}
}

func (h *Handler) scriptByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/scripts/")
	if id == "" {
		httpjson.BadRequest(w, apiError("script id required"))
		return
	}
	if r.Method != http.MethodDelete {
		httpjson.MethodNotAllowed(w)
		return
	}
	if h.store != nil {
		_ = h.store.DeleteScript(context.Background(), id)
	}
	httpjson.WriteJSON(w, http.StatusOK, api.AcceptedResponse{Status: "deleted"})
}

// ── Alert Rules ───────────────────────────────────────────────────────────────

func (h *Handler) alertRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			tenantID = "dev-tenant-001"
		}
		if h.store != nil {
			rules, err := h.store.ListAlertRules(context.Background(), tenantID)
			if err == nil {
				httpjson.WriteJSON(w, http.StatusOK, rules)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusOK, []store.AlertRule{})
	case http.MethodPost:
		var req store.AlertRule
		if err := httpjson.Decode(r, &req); err != nil {
			httpjson.BadRequest(w, err)
			return
		}
		if req.TenantID == "" {
			req.TenantID = "dev-tenant-001"
		}
		if h.store == nil {
			httpjson.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Error: "no database"})
			return
		}
		rule, err := h.store.CreateAlertRule(context.Background(), req)
		if err != nil {
			httpjson.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}
		// Evaluate immediately
		go func() { _ = h.store.EvaluateAlerts(context.Background()) }()
		httpjson.WriteJSON(w, http.StatusCreated, rule)
	default:
		httpjson.MethodNotAllowed(w)
	}
}

func (h *Handler) alertRuleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/alert-rules/")
	if id == "" {
		httpjson.BadRequest(w, apiError("rule id required"))
		return
	}
	if r.Method != http.MethodDelete {
		httpjson.MethodNotAllowed(w)
		return
	}
	if h.store != nil {
		_ = h.store.DeleteAlertRule(context.Background(), id)
	}
	httpjson.WriteJSON(w, http.StatusOK, api.AcceptedResponse{Status: "deleted"})
}

func (h *Handler) alertEvents(w http.ResponseWriter, _ *http.Request) {
	tenantID := "dev-tenant-001"
	if h.store != nil {
		events, err := h.store.ListAlertEvents(context.Background(), tenantID, 50)
		if err == nil {
			httpjson.WriteJSON(w, http.StatusOK, events)
			return
		}
	}
	httpjson.WriteJSON(w, http.StatusOK, []store.AlertEvent{})
}

type apiError string

func (e apiError) Error() string { return string(e) }
