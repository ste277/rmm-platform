package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func Dial(ctx context.Context, endpoint string) (*Client, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, endpoint, http.Header{})
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) WriteJSON(payload any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(payload)
}

func (c *Client) ReadJSON(out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(msg, out)
}

func (c *Client) Close() error {
	return c.conn.Close()
}
