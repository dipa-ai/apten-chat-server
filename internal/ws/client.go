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
	writeWait      = 10 * time.Second
	pongWait       = 30 * time.Second
	pingPeriod     = 25 * time.Second
	maxMsgSize     = 4096
	wsRateLimit    = 30 // events per minute
	wsRateBurst    = 5
)

type Client struct {
	UserID int64
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan Event
	ctx    context.Context
	cancel context.CancelFunc
}

func NewClient(hub *Hub, conn *websocket.Conn, userID int64) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		UserID: userID,
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan Event, 256),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *Client) ReadPump(handler func(client *Client, evt Event)) {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close(websocket.StatusNormalClosure, "")
		c.cancel()
	}()

	c.Conn.SetReadLimit(maxMsgSize)

	limiter := rate.NewLimiter(rate.Limit(float64(wsRateLimit)/60.0), wsRateBurst)

	for {
		_, data, err := c.Conn.Read(c.ctx)
		if err != nil {
			break
		}

		if !limiter.Allow() {
			log.Printf("ws: rate limited user %d", c.UserID)
			continue
		}

		var evt Event
		if err := json.Unmarshal(data, &evt); err != nil {
			log.Printf("ws: invalid event from user %d: %v", c.UserID, err)
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
