package httpapi

import (
	"context"
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

	mux.HandleFunc("/api/v1/sessions", func(w http.ResponseWriter, _ *http.Request) {
		httpjson.WriteJSON(w, http.StatusOK, broker.Sessions())
	})

	mux.HandleFunc("/api/v1/agent-commands/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpjson.MethodNotAllowed(w)
			return
		}

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

		// 1. Persist first to get the real command ID
		resp := api.CommandResponse{CommandID: "dev-cmd", Status: "dispatched"}
		if db != nil {
			if r, err := db.CreateCommand(context.Background(), cmd); err == nil {
				resp = r
			}
		}

		// 2. Send that same ID to the agent so results can be matched
		if err := broker.SendCommandWithID(agentID, resp.CommandID, cmd); err != nil {
			httpjson.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}

		resp.Status = "dispatched"
		httpjson.WriteJSON(w, http.StatusAccepted, resp)
	})

	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: "ok", Service: "broker"})
}

type apiError string

func (e apiError) Error() string { return string(e) }
