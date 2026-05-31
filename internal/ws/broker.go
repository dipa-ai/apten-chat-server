package ws

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

// brokerChannel is the Redis Pub/Sub channel that carries WebSocket events
// between server replicas.
const brokerChannel = "apten-chat:ws-events"

// BrokerEvent is the envelope published to Redis. Origin identifies the
// publishing instance so it can ignore its own messages on the way back. When
// Broadcast is true the event is destined for every client (e.g. presence) and
// UserIDs is empty; otherwise it targets the listed users.
type BrokerEvent struct {
	Origin    string  `json:"origin"`
	Broadcast bool    `json:"broadcast,omitempty"`
	UserIDs   []int64 `json:"user_ids,omitempty"`
	Event     Event   `json:"event"`
}

// Broker bridges WebSocket events across replicas over Redis Pub/Sub. Each
// instance publishes the events it originates and, via Run, fans events from
// other instances out to its own locally-connected clients. onEvent delivers
// targeted events; onBroadcast delivers broadcast events (presence) to all
// local clients.
type Broker struct {
	client      *redis.Client
	origin      string
	onEvent     func(userIDs []int64, evt Event)
	onBroadcast func(evt Event)
}

func NewBroker(client *redis.Client, origin string, onEvent func(userIDs []int64, evt Event), onBroadcast func(evt Event)) *Broker {
	return &Broker{client: client, origin: origin, onEvent: onEvent, onBroadcast: onBroadcast}
}

// Publish sends a targeted event to the other replicas. Failures are logged but
// not fatal: local delivery has already happened, so cross-replica delivery is
// best-effort.
func (b *Broker) Publish(ctx context.Context, userIDs []int64, evt Event) {
	b.publish(ctx, BrokerEvent{Origin: b.origin, UserIDs: userIDs, Event: evt})
}

// PublishBroadcast sends an all-clients event (presence) to the other replicas.
func (b *Broker) PublishBroadcast(ctx context.Context, evt Event) {
	b.publish(ctx, BrokerEvent{Origin: b.origin, Broadcast: true, Event: evt})
}

func (b *Broker) publish(ctx context.Context, event BrokerEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("ws broker: marshal: %v", err)
		return
	}
	if err := b.client.Publish(ctx, brokerChannel, payload).Err(); err != nil {
		log.Printf("ws broker: publish: %v", err)
	}
}

// Run subscribes to the broker channel and delivers events from other replicas
// to local clients via onEvent. It blocks until ctx is cancelled.
func (b *Broker) Run(ctx context.Context) {
	pubsub := b.client.Subscribe(ctx, brokerChannel)
	defer pubsub.Close()
	for msg := range pubsub.Channel() {
		b.handleMessage([]byte(msg.Payload))
	}
}

// handleMessage decodes a published payload and, unless this instance was the
// origin, fans the event out to local clients.
func (b *Broker) handleMessage(payload []byte) {
	var event BrokerEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("ws broker: unmarshal: %v", err)
		return
	}
	// Skip events this instance published — they were already delivered
	// locally, and re-delivering them would double-send.
	if event.Origin == b.origin {
		return
	}
	if event.Broadcast {
		b.onBroadcast(event.Event)
		return
	}
	b.onEvent(event.UserIDs, event.Event)
}
