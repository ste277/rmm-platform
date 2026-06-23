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
	"time"
)

const (
	writeTimeout = 10 * time.Second
	readTimeout  = 45 * time.Second
)

// Client is a WebSocket client compatible with the custom broker in broker.go.
// It uses a native HTTP upgrade handshake instead of gorilla/websocket.
//
// readMu and writeMu are separate locks (rather than one shared mutex)
// because ReadJSON blocks on a network read that can legitimately take a
// long time — the broker only pushes data when it has something to send.
// With one shared mutex, the command receiver's continuous blocking
// Receive() loop holds the lock for the entire duration of that read,
// starving every other goroutine (heartbeat, inventory, telemetry,
// compliance) that needs WriteJSON. That was confirmed via SIGQUIT
// goroutine dump: all four senders parked on the same mutex for hours
// while the reader held it inside a blocking syscall.
type Client struct {
	conn    net.Conn
	rw      *bufio.ReadWriter
	readMu  sync.Mutex
	writeMu sync.Mutex
}

// Dial connects to a WebSocket endpoint and performs the upgrade handshake.
func Dial(ctx context.Context, endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	var d net.Dialer
	d.Timeout = 10 * time.Second
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("dial tcp: %w", err)
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("generate ws key: %w", err)
	}
	wsKey := base64.StdEncoding.EncodeToString(keyBytes)

	requestURI := u.RequestURI()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
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

	_ = conn.SetReadDeadline(time.Now().Add(writeTimeout))
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

	_ = conn.SetDeadline(time.Time{})

	return &Client{conn: conn, rw: rw}, nil
}

// WriteJSON marshals v and sends it as a WebSocket text frame.
// Uses writeMu (separate from readMu) so a concurrent blocking ReadJSON
// call never prevents a write from going out. A write deadline ensures a
// stalled/half-open connection fails fast instead of blocking forever.
func (c *Client) WriteJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	defer func() { _ = c.conn.SetWriteDeadline(time.Time{}) }()

	return writeDataFrame(c.rw.Writer, 0x1, payload)
}

// ReadJSON reads the next WebSocket text frame and unmarshals it into v.
// Uses readMu (separate from writeMu) so this can legitimately block for a
// long time waiting on data without starving WriteJSON callers. A read
// deadline prevents an idle connection from blocking forever on a peer
// that has silently gone away.
func (c *Client) ReadJSON(v any) error {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if err := c.conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}
	defer func() { _ = c.conn.SetReadDeadline(time.Time{}) }()

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
