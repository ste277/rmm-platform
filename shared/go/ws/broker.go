package ws

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"example.com/rmm-shared/api"
	"example.com/rmm-shared/store"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Session struct {
	AgentID     string
	RemoteAddr  string
	ConnectedAt time.Time
	conn        net.Conn
	mu          sync.Mutex
}

type Broker struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	store    *store.Store
}

func NewBroker(db *store.Store) *Broker {
	return &Broker{sessions: make(map[string]*Session), store: db}
}

func (b *Broker) Sessions() []api.SessionSummary {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]api.SessionSummary, 0, len(b.sessions))
	for _, session := range b.sessions {
		out = append(out, api.SessionSummary{
			AgentID:     session.AgentID,
			RemoteAddr:  session.RemoteAddr,
			ConnectedAt: session.ConnectedAt.Format(time.RFC3339),
		})
	}
	return out
}

// SendCommand pushes a command to a connected agent over its WebSocket session.
// Returns an error if the agent is not currently connected.
func (b *Broker) SendCommand(agentID string, cmd api.CommandCreateRequest) error {
	b.mu.RLock()
	session, ok := b.sessions[agentID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent %q is not connected", agentID)
	}

	payload, err := json.Marshal(map[string]any{
		"type":           "command",
		"command_id":     fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		"command_type":   cmd.CommandType,
		"script_body":    cmd.ScriptBody,
		"args":           cmd.Args,
		"timeout_seconds": cmd.TimeoutSec,
	})
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	return session.writeText(string(payload))
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !headerContainsToken(r.Header, "Connection", "Upgrade") || !headerContainsToken(r.Header, "Upgrade", "websocket") {
		http.Error(w, "websocket upgrade required", http.StatusUpgradeRequired)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "server does not support websocket", http.StatusInternalServerError)
		return
	}

	conn, rw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	acceptKey := computeAcceptKey(key)
	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		acceptKey,
	)
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return
	}

	session := &Session{
		AgentID:     agentID,
		RemoteAddr:  conn.RemoteAddr().String(),
		ConnectedAt: time.Now().UTC(),
		conn:        conn,
	}

	b.mu.Lock()
	b.sessions[agentID] = session
	b.mu.Unlock()

	log.Printf("agent connected: agent_id=%s remote=%s", agentID, session.RemoteAddr)

	defer func() {
		b.mu.Lock()
		delete(b.sessions, agentID)
		b.mu.Unlock()
		_ = conn.Close()
		log.Printf("agent disconnected: agent_id=%s", agentID)
	}()

	_ = session.writeText(`{"type":"welcome","message":"connected"}`)

	for {
		payload, opcode, err := readFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("read error for agent %s: %v", agentID, err)
			}
			return
		}

		switch opcode {
		case 0x8: // close
			_ = writeControlFrame(conn, 0x8, nil)
			return
		case 0x9: // ping → pong
			_ = writeControlFrame(conn, 0xA, payload)
		case 0x1: // text frame
			trimmed := strings.TrimSpace(string(payload))
			if trimmed == "" {
				trimmed = "{}"
			}
			if err := b.handleAgentMessage(session.AgentID, trimmed); err != nil {
				log.Printf("broker ingest error for %s: %v", session.AgentID, err)
			}
			_ = session.writeText(fmt.Sprintf(`{"type":"ack","agent_id":"%s"}`, session.AgentID))
		}
	}
}

func (b *Broker) handleAgentMessage(sessionAgentID, payload string) error {
	if b.store == nil {
		return nil
	}

	var req api.IngestRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return err
	}
	if req.AgentID == "" {
		req.AgentID = sessionAgentID
	}
	return b.store.ProcessIngest(context.Background(), req)
}

func (s *Session) writeText(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeDataFrame(s.conn, 0x1, []byte(message))
}

func computeAcceptKey(key string) string {
	hash := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func headerContainsToken(header http.Header, key, token string) bool {
	for _, part := range strings.Split(header.Get(key), ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func readFrame(r io.Reader) ([]byte, byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, 0, err
	}

	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := int(header[1] & 0x7F)

	switch length {
	case 126:
		var ext uint16
		if err := binary.Read(r, binary.BigEndian, &ext); err != nil {
			return nil, 0, err
		}
		length = int(ext)
	case 127:
		var ext uint64
		if err := binary.Read(r, binary.BigEndian, &ext); err != nil {
			return nil, 0, err
		}
		length = int(ext)
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return nil, 0, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return payload, opcode, nil
}

func writeControlFrame(w io.Writer, opcode byte, payload []byte) error {
	return writeDataFrame(w, opcode, payload)
}

func writeDataFrame(w io.Writer, opcode byte, payload []byte) error {
	bw := bufio.NewWriter(w)
	first := byte(0x80 | opcode)
	if err := bw.WriteByte(first); err != nil {
		return err
	}

	length := len(payload)
	switch {
	case length < 126:
		if err := bw.WriteByte(byte(length)); err != nil {
			return err
		}
	case length <= 65535:
		if err := bw.WriteByte(126); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.BigEndian, uint16(length)); err != nil {
			return err
		}
	default:
		if err := bw.WriteByte(127); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.BigEndian, uint64(length)); err != nil {
			return err
		}
	}

	if _, err := bw.Write(payload); err != nil {
		return err
	}
	return bw.Flush()
}
