package httpapi

import (
	"net/http"
	"strings"

	"example.com/rmm-shared/api"
	"example.com/rmm-shared/httpjson"
	"example.com/rmm-shared/store"
	"example.com/rmm-shared/ws"
)

func NewMux(db *store.Store) *http.ServeMux {
	broker := ws.NewBroker(db)
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthz)
	mux.Handle("/ws", broker)

	// List active sessions
	mux.HandleFunc("/api/v1/sessions", func(w http.ResponseWriter, _ *http.Request) {
		httpjson.WriteJSON(w, http.StatusOK, broker.Sessions())
	})

	// Push a command to a connected agent: POST /api/v1/agent-commands/{agent_id}
	mux.HandleFunc("/api/v1/agent-commands/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpjson.MethodNotAllowed(w)
			return
		}

		// Extract agent_id from path: /api/v1/commands/{agent_id}
		agentID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent-commands/")
		if agentID == "" {
			httpjson.BadRequest(w, apiError("agent_id is required in path"))
			return
		}

		var cmd api.CommandCreateRequest
		if err := httpjson.Decode(r, &cmd); err != nil {
			httpjson.BadRequest(w, err)
			return
		}
		cmd.AgentID = agentID

		if err := broker.SendCommand(agentID, cmd); err != nil {
			httpjson.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}

		httpjson.WriteJSON(w, http.StatusAccepted, api.CommandResponse{
			CommandID: "pending",
			Status:    "dispatched",
		})
	})

	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "broker"})
}

type apiError string

func (e apiError) Error() string { return string(e) }
