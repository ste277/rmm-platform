package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"example.com/rmm-shared/ws"
)

type Client struct {
	serverURL string
	http      *http.Client
	ws        *ws.Client
	mu        sync.RWMutex
	connected bool
}

func NewClient(serverURL string) *Client {
	return &Client{
		serverURL: serverURL,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	log.Printf("connecting agent transport to %s", c.serverURL)
	if strings.HasPrefix(c.serverURL, "ws://") || strings.HasPrefix(c.serverURL, "wss://") {
		socket, err := ws.Dial(ctx, c.serverURL)
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.ws = socket
		c.connected = true
		c.mu.Unlock()
		return nil
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
	return nil
}

func (c *Client) Send(msg any) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()
	if !connected {
		return nil
	}

	if c.ws != nil {
		return c.ws.WriteJSON(msg)
	}

	if strings.HasPrefix(c.serverURL, "http://") || strings.HasPrefix(c.serverURL, "https://") {
		payload, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		endpoint := strings.TrimRight(c.serverURL, "/") + "/api/v1/ingest"
		resp, err := c.http.Post(endpoint, "application/json", bytes.NewReader(payload))
		if err == nil && resp.Body != nil {
			_ = resp.Body.Close()
			return nil
		}
		log.Printf("http transport unavailable, falling back to local log")
		log.Printf("sending message locally: %s", string(payload))
		return nil
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	log.Printf("sending message locally: %s", string(payload))
	return nil
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
