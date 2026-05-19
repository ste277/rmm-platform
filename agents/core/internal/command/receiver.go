package command

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"strings"
	"time"

	"rmm-agent/internal/transport"
)

// IncomingCommand is pushed from the command-service to the agent over WebSocket.
type IncomingCommand struct {
	CommandID   string   `json:"command_id"`
	CommandType string   `json:"command_type"` // "ping" | "shell" | "script"
	ScriptBody  string   `json:"script_body,omitempty"`
	Args        []string `json:"args,omitempty"`
	TimeoutSec  int      `json:"timeout_seconds,omitempty"`
}

// CommandResult is sent back to the broker after execution.
type CommandResult struct {
	Type      string `json:"type"` // always "command_result"
	CommandID string `json:"command_id"`
	AgentID   string `json:"agent_id"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	RanAt     int64  `json:"ran_at_unix"`
}

// Receiver listens on the WebSocket for incoming commands and dispatches them.
type Receiver struct {
	transport *transport.Client
	agentID   string
}

func NewReceiver(t *transport.Client, agentID string) *Receiver {
	return &Receiver{transport: t, agentID: agentID}
}

// Start blocks reading from the WebSocket until ctx is cancelled.
// If the transport is not a WebSocket connection it exits immediately —
// command dispatch requires a persistent bidirectional channel.
func (r *Receiver) Start(ctx context.Context) {
	if !r.transport.IsWebSocket() {
		log.Println("command receiver: skipping — WebSocket connection required")
		return
	}

	log.Println("command receiver started")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			raw, err := r.transport.Receive()
			if err != nil {
				log.Printf("command receive error: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				continue
			}

			// Peek at type field — only handle "command" messages
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Type != "command" {
				continue
			}

			var cmd IncomingCommand
			if err := json.Unmarshal(raw, &cmd); err != nil {
				log.Printf("malformed command message: %v", err)
				continue
			}

			// Execute concurrently so the receive loop stays unblocked
			go r.execute(ctx, cmd)
		}
	}
}

func (r *Receiver) execute(ctx context.Context, cmd IncomingCommand) {
	log.Printf("executing command: id=%s type=%s", cmd.CommandID, cmd.CommandType)

	timeout := time.Duration(cmd.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := CommandResult{
		Type:      "command_result",
		CommandID: cmd.CommandID,
		AgentID:   r.agentID,
		RanAt:     time.Now().Unix(),
	}

	switch cmd.CommandType {
	case "ping":
		result.Output = "pong"
		result.ExitCode = 0

	case "shell", "script":
		var c *exec.Cmd
		if cmd.CommandType == "script" && cmd.ScriptBody != "" {
			c = exec.CommandContext(execCtx, "sh", "-c", cmd.ScriptBody)
		} else if len(cmd.Args) > 0 {
			c = exec.CommandContext(execCtx, cmd.Args[0], cmd.Args[1:]...)
		} else {
			result.Error = "no script_body or args provided"
			result.ExitCode = 1
			break
		}

		out, err := c.CombinedOutput()
		result.Output = strings.TrimSpace(string(out))
		if err != nil {
			result.Error = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = 1
			}
		}

	default:
		result.Error = "unknown command type: " + cmd.CommandType
		result.ExitCode = 1
	}

	if err := r.transport.Send(result); err != nil {
		log.Printf("failed to send command result: %v", err)
	}
}
