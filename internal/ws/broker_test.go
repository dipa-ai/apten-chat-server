package ws

import (
	"encoding/json"
	"testing"
)

func TestBroker_HandleMessage_FansOutRemoteEvent(t *testing.T) {
	var gotUserIDs []int64
	var gotEvent Event
	called := 0
	b := NewBroker(nil, "instance-A", func(userIDs []int64, evt Event) {
		called++
		gotUserIDs = userIDs
		gotEvent = evt
	}, func(Event) {})

	payload, _ := json.Marshal(BrokerEvent{
		Origin:  "instance-B", // a different replica
		UserIDs: []int64{1, 2},
		Event:   Event{Type: "message.new", Payload: json.RawMessage(`{"id":7}`)},
	})
	b.handleMessage(payload)

	if called != 1 {
		t.Fatalf("onEvent called %d times, want 1", called)
	}
	if len(gotUserIDs) != 2 || gotUserIDs[0] != 1 || gotUserIDs[1] != 2 {
		t.Errorf("userIDs = %v, want [1 2]", gotUserIDs)
	}
	if gotEvent.Type != "message.new" {
		t.Errorf("event type = %q, want message.new", gotEvent.Type)
	}
}

func TestBroker_HandleMessage_SkipsOwnOrigin(t *testing.T) {
	called := 0
	b := NewBroker(nil, "instance-A", func([]int64, Event) { called++ }, func(Event) { called++ })

	payload, _ := json.Marshal(BrokerEvent{
		Origin:  "instance-A", // same instance — must be ignored
		UserIDs: []int64{1},
		Event:   Event{Type: "message.new"},
	})
	b.handleMessage(payload)

	if called != 0 {
		t.Errorf("onEvent called %d times for own-origin event, want 0", called)
	}
}

func TestBroker_HandleMessage_IgnoresMalformed(t *testing.T) {
	called := 0
	b := NewBroker(nil, "instance-A", func([]int64, Event) { called++ }, func(Event) { called++ })

	b.handleMessage([]byte("not json")) // must not panic

	if called != 0 {
		t.Errorf("onEvent called %d times for malformed payload, want 0", called)
	}
}

// A broadcast event (e.g. presence) from another replica must be delivered via
// onBroadcast, not onEvent.
func TestBroker_HandleMessage_RoutesBroadcast(t *testing.T) {
	targeted := 0
	broadcast := 0
	b := NewBroker(nil, "instance-A",
		func([]int64, Event) { targeted++ },
		func(evt Event) {
			broadcast++
			if evt.Type != "presence.update" {
				t.Errorf("broadcast event type = %q, want presence.update", evt.Type)
			}
		},
	)

	payload, _ := json.Marshal(BrokerEvent{
		Origin:    "instance-B",
		Broadcast: true,
		Event:     Event{Type: "presence.update"},
	})
	b.handleMessage(payload)

	if broadcast != 1 {
		t.Errorf("onBroadcast called %d times, want 1", broadcast)
	}
	if targeted != 0 {
		t.Errorf("onEvent called %d times for a broadcast, want 0", targeted)
	}
}
