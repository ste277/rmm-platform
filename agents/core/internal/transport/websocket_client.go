package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"example.com/rmm-shared/ws"
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 60 * time.Second
)

type Client struct {
	serverURL string
	agentID   string
	http      *http.Client
	ws        *ws.Client
	mu        sync.RWMutex
	connected bool
}

func NewClient(serverURL string, agentID string) *Client {
	return &Client{
		serverURL: serverURL,
		agentID:   agentID,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Connect(ctx context.Context) error {
	log.Printf("connecting agent transport to %s", c.serverURL)

	if strings.HasPrefix(c.serverURL, "ws://") || strings.HasPrefix(c.serverURL, "wss://") {
		return c.connectWebSocket(ctx)
	}

	if strings.HasPrefix(c.serverURL, "http://") || strings.HasPrefix(c.serverURL, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.serverURL, "/")+"/healthz", nil)
		if err == nil {
			resp, err := c.http.Do(req)
			if err == nil && resp.Body != nil {
				_ = resp.Body.Close()
				log.Printf("transport health probe status=%s", resp.Status)
			}
		}
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

func (c *Client) connectWebSocket(ctx context.Context) error {
	wsURL := c.wsURL()
	socket, err := ws.Dial(ctx, wsURL)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.mu.Lock()
	c.ws = socket
	c.connected = true
	c.mu.Unlock()

	go c.reconnectLoop(ctx, wsURL)
	return nil
}

// wsURL builds the full WebSocket URL including agent_id query param.
func (c *Client) wsURL() string {
	u := c.serverURL
	if c.agentID != "" {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "agent_id=" + c.agentID
	}
	return u
}

// reconnectLoop watches for a dropped WebSocket and reconnects with exponential backoff.
func (c *Client) reconnectLoop(ctx context.Context, wsURL string) {
	backoff := initialBackoff
	for {
		// Check every 5 seconds whether the socket is still alive
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}

		c.mu.RLock()
		socket := c.ws
		c.mu.RUnlock()

		if socket != nil {
			backoff = initialBackoff // still connected — reset backoff
			continue
		}

		// Socket is nil — attempt reconnect
		log.Printf("websocket disconnected, reconnecting in %s...", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		newSocket, err := ws.Dial(ctx, wsURL)
		if err != nil {
			log.Printf("reconnect failed: %v", err)
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}

		c.mu.Lock()
		c.ws = newSocket
		c.connected = true
		c.mu.Unlock()

		log.Printf("websocket reconnected successfully")
		backoff = initialBackoff
	}
}

func (c *Client) Send(msg any) error {
	c.mu.RLock()
	connected := c.connected
	socket := c.ws
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("transport not connected")
	}

	if socket != nil {
		if err := socket.WriteJSON(msg); err != nil {
			c.mu.Lock()
			c.ws = nil
			c.mu.Unlock()
			return err
		}
		return nil
	}

	if strings.HasPrefix(c.serverURL, "http://") || strings.HasPrefix(c.serverURL, "https://") {
		payload, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		endpoint := strings.TrimRight(c.serverURL, "/") + "/api/v1/ingest"
		resp, err := c.http.Post(endpoint, "application/json", bytes.NewReader(payload))
		if err != nil {
			log.Printf("http ingest unavailable, logging locally: %v", err)
			log.Printf("message: %s", string(payload))
			return nil
		}
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	log.Printf("sending message locally: %s", string(payload))
	return nil
}

// Receive reads a raw JSON message from the WebSocket.
// Only available when connected via ws:// or wss://.
func (c *Client) Receive() ([]byte, error) {
	c.mu.RLock()
	socket := c.ws
	c.mu.RUnlock()

	if socket == nil {
		return nil, fmt.Errorf("receive requires a WebSocket connection")
	}

	var raw json.RawMessage
	if err := socket.ReadJSON(&raw); err != nil {
		c.mu.Lock()
		c.ws = nil
		c.mu.Unlock()
		return nil, err
	}
	return raw, nil
}

// IsWebSocket reports whether the transport is currently connected via WebSocket.
func (c *Client) IsWebSocket() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ws != nil
}

func (c *Client) Close(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
	log.Printf("transport closed")
	return nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
