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
	mux.HandleFunc("/api/v1/commands", h.commands)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "command"})
}

func (h *Handler) commands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.store != nil {
			records, err := h.store.ListCommands(context.Background())
			if err == nil {
				httpjson.WriteJSON(w, http.StatusOK, records)
				return
			}
		}
		httpjson.WriteJSON(w, http.StatusOK, []api.CommandRecord{
			{CommandID: "cmd-dev-001", AgentID: "dev-agent-001", CommandType: "script", Status: "queued"},
		})
	case http.MethodPost:
		var req api.CommandCreateRequest
		if err := httpjson.Decode(r, &req); err != nil {
			httpjson.BadRequest(w, err)
			return
		}
		if req.AgentID == "" || req.CommandType == "" {
			httpjson.BadRequest(w, apiError("agent_id and command_type are required"))
			return
		}
		if h.store != nil {
			resp, err := h.store.CreateCommand(context.Background(), req)
			if err != nil {
				httpjson.BadRequest(w, err)
				return
			}
			httpjson.WriteJSON(w, http.StatusAccepted, resp)
			return
		}
		httpjson.WriteJSON(w, http.StatusAccepted, api.CommandResponse{CommandID: "cmd-dev-001", Status: "queued"})
	default:
		httpjson.MethodNotAllowed(w)
	}
}

type apiError string

func (e apiError) Error() string {
	return string(e)
}
