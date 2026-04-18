package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 25 * time.Second
	maxMsgSize = 16 * 1024

	// message.send — steady 0.5/s, burst 10. Enough for a human typing
	// fast without letting a misbehaving client flood the chat.
	sendRatePerSec = 0.5
	sendRateBurst  = 10

	// typing.*, message.read — steady 3/s, burst 60. High-frequency events
	// that must never block an actual send.
	miscRatePerSec = 3.0
	miscRateBurst  = 60
)

type Client struct {
	UserID int64
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan Event

	sendLimiter *rate.Limiter
	miscLimiter *rate.Limiter

	ctx    context.Context
	cancel context.CancelFunc
}

func NewClient(hub *Hub, conn *websocket.Conn, userID int64) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		UserID:      userID,
		Hub:         hub,
		Conn:        conn,
		Send:        make(chan Event, 256),
		sendLimiter: rate.NewLimiter(rate.Limit(sendRatePerSec), sendRateBurst),
		miscLimiter: rate.NewLimiter(rate.Limit(miscRatePerSec), miscRateBurst),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Disconnect cancels the client context. ReadPump will exit and the
// deferred Unregister will tear down the connection.
func (c *Client) Disconnect() {
	c.cancel()
}

// TrySend enqueues an event on the client's send buffer without blocking.
// Returns false if the buffer is full or the client is shutting down.
func (c *Client) TrySend(evt Event) bool {
	select {
	case <-c.ctx.Done():
		return false
	case c.Send <- evt:
		return true
	default:
		return false
	}
}

func (c *Client) sendError(clientID, reason string) {
	evt, err := NewEvent("message.error", MessageErrorPayload{
		ClientID: clientID,
		Reason:   reason,
	})
	if err != nil {
		return
	}
	c.TrySend(evt)
}

func (c *Client) ReadPump(handler func(client *Client, evt Event)) {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close(websocket.StatusNormalClosure, "")
		c.cancel()
	}()

	c.Conn.SetReadLimit(maxMsgSize)

	for {
		_, data, err := c.Conn.Read(c.ctx)
		if err != nil {
			return
		}

		var evt Event
		if err := json.Unmarshal(data, &evt); err != nil {
			log.Printf("ws: invalid event from user %d: %v", c.UserID, err)
			continue
		}

		var allow bool
		switch evt.Type {
		case "message.send":
			allow = c.sendLimiter.Allow()
		default:
			allow = c.miscLimiter.Allow()
		}
		if !allow {
			log.Printf("ws: rate limited user %d (event=%s)", c.UserID, evt.Type)
			if evt.Type == "message.send" {
				var p MessageSendPayload
				if json.Unmarshal(evt.Payload, &p) == nil && p.ClientID != "" {
					c.sendError(p.ClientID, "rate_limited")
				}
			}
			continue
		}

		handler(c, evt)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case evt, ok := <-c.Send:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(c.ctx, writeWait)
			data, err := json.Marshal(evt)
			if err != nil {
				cancel()
				continue
			}
			err = c.Conn.Write(ctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, writeWait)
			err := c.Conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

var _ = pongWait // reserved for future read-deadline wiring
