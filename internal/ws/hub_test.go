package ws

import "testing"

func TestSendLocal_DeliversToConnectedUser(t *testing.T) {
	h := NewHub()
	c := &Client{UserID: 5, Send: make(chan Event, 1)}
	h.clients[5] = map[*Client]bool{c: true}

	h.SendLocal([]int64{5}, Event{Type: "ping"})

	select {
	case got := <-c.Send:
		if got.Type != "ping" {
			t.Errorf("event type = %q, want ping", got.Type)
		}
	default:
		t.Fatal("expected an event to be delivered to the client")
	}
}

// Send must deliver locally even when no broker is configured (the common
// single-replica case) without panicking on the nil Broker.
func TestSend_WithoutBroker_DeliversLocally(t *testing.T) {
	h := NewHub()
	c := &Client{UserID: 9, Send: make(chan Event, 1)}
	h.clients[9] = map[*Client]bool{c: true}

	h.Send([]int64{9}, Event{Type: "pong"})

	select {
	case got := <-c.Send:
		if got.Type != "pong" {
			t.Errorf("event type = %q, want pong", got.Type)
		}
	default:
		t.Fatal("expected an event to be delivered to the client")
	}
}
