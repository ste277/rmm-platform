package ws

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Client is a WebSocket client compatible with the custom broker in broker.go.
// It uses a native HTTP upgrade handshake instead of gorilla/websocket.
type Client struct {
	conn net.Conn
	rw   *bufio.ReadWriter
	mu   sync.Mutex
}

// Dial connects to a WebSocket endpoint and performs the upgrade handshake.
func Dial(ctx context.Context, endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	// Resolve host and port
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// Open raw TCP connection
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("dial tcp: %w", err)
	}

	// Generate Sec-WebSocket-Key
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("generate ws key: %w", err)
	}
	wsKey := base64.StdEncoding.EncodeToString(keyBytes)

	// Build the HTTP upgrade request path (preserve query string for agent_id etc.)
	requestURI := u.RequestURI()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// Write HTTP upgrade request
	lines := []string{
		fmt.Sprintf("GET %s HTTP/1.1", requestURI),
		fmt.Sprintf("Host: %s", u.Host),
		"Upgrade: websocket",
		"Connection: Upgrade",
		fmt.Sprintf("Sec-WebSocket-Key: %s", wsKey),
		"Sec-WebSocket-Version: 13",
		"\r\n",
	}
	if _, err := rw.WriteString(strings.Join(lines, "\r\n")); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write upgrade request: %w", err)
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("flush upgrade request: %w", err)
	}

	// Read HTTP response
	resp, err := http.ReadResponse(rw.Reader, &http.Request{Method: "GET"})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read upgrade response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected status: %d %s", resp.StatusCode, resp.Status)
	}

	return &Client{conn: conn, rw: rw}, nil
}

// WriteJSON marshals v and sends it as a WebSocket text frame.
func (c *Client) WriteJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return writeDataFrame(c.rw.Writer, 0x1, payload)
}

// ReadJSON reads the next WebSocket text frame and unmarshals it into v.
func (c *Client) ReadJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, _, err := readFrame(c.rw.Reader)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, v)
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
