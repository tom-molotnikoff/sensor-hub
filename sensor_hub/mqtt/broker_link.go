package mqtt

import (
	"fmt"
	"sync"
	"time"

	gen "example/sensorHub/gen"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// messageReceiver is called with every message on a subscribed topic.
type messageReceiver func(topic string, payload []byte)

// brokerLink is the hub's connection to one broker: a Paho client over the
// network for an external broker, or mochi's inline client for the embedded
// broker, which needs no credential and is not subject to the client ACL.
type brokerLink interface {
	subscribe(sub gen.MQTTSubscription, receive messageReceiver) error
	unsubscribe(sub gen.MQTTSubscription) error
	publish(topic string, payload []byte, qos byte) error
	connected() bool
	close()
}

type pahoLink struct {
	client pahomqtt.Client
}

func (l *pahoLink) subscribe(sub gen.MQTTSubscription, receive messageReceiver) error {
	token := l.client.Subscribe(sub.TopicPattern, 0, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		receive(msg.Topic(), msg.Payload())
	})
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (l *pahoLink) unsubscribe(sub gen.MQTTSubscription) error {
	token := l.client.Unsubscribe(sub.TopicPattern)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (l *pahoLink) publish(topic string, payload []byte, qos byte) error {
	token := l.client.Publish(topic, qos, false, payload)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (l *pahoLink) connected() bool {
	return l.client.IsConnected()
}

func (l *pahoLink) close() {
	l.client.Disconnect(250)
}

type inlineMessage struct {
	receive messageReceiver
	topic   string
	payload []byte
}

// inlineLink reaches the embedded broker in-process. Inline subscriptions are
// keyed by subscription ID, so two subscriptions on one filter stay separate.
//
// Mochi calls inline handlers on the publishing device's connection, so they
// only queue the message, and one goroutine handles the queue in arrival
// order: a slow database write does not stall the device. The queue is
// unbounded because the hub's own publishes are delivered back to it, and a
// full queue would leave a handler that publishes waiting on itself.
type inlineLink struct {
	server *mochi.Server
	wake   chan struct{}
	done   chan struct{}

	queueMu sync.Mutex
	queue   []inlineMessage

	mu            sync.Mutex
	subscriptions map[int]string // subscription ID → topic filter
	closeOnce     sync.Once
}

func newInlineLink(server *mochi.Server) *inlineLink {
	l := &inlineLink{
		server:        server,
		wake:          make(chan struct{}, 1),
		done:          make(chan struct{}),
		subscriptions: make(map[int]string),
	}
	go l.deliver()
	return l
}

func (l *inlineLink) enqueue(msg inlineMessage) {
	l.queueMu.Lock()
	l.queue = append(l.queue, msg)
	l.queueMu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *inlineLink) deliver() {
	for {
		select {
		case <-l.wake:
		case <-l.done:
			return
		}
		for {
			l.queueMu.Lock()
			if len(l.queue) == 0 {
				l.queue = nil
				l.queueMu.Unlock()
				break
			}
			msg := l.queue[0]
			l.queue = l.queue[1:]
			l.queueMu.Unlock()
			msg.receive(msg.topic, msg.payload)
		}
	}
}

func (l *inlineLink) subscribe(sub gen.MQTTSubscription, receive messageReceiver) error {
	if sub.Id == nil {
		return fmt.Errorf("subscription to %s has no id", sub.TopicPattern)
	}
	err := l.server.Subscribe(sub.TopicPattern, *sub.Id, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		l.enqueue(inlineMessage{receive: receive, topic: pk.TopicName, payload: append([]byte(nil), pk.Payload...)})
	})
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.subscriptions[*sub.Id] = sub.TopicPattern
	l.mu.Unlock()
	return nil
}

func (l *inlineLink) unsubscribe(sub gen.MQTTSubscription) error {
	if sub.Id == nil {
		return fmt.Errorf("subscription to %s has no id", sub.TopicPattern)
	}
	l.mu.Lock()
	delete(l.subscriptions, *sub.Id)
	l.mu.Unlock()
	return l.server.Unsubscribe(sub.TopicPattern, *sub.Id)
}

func (l *inlineLink) publish(topic string, payload []byte, qos byte) error {
	return l.server.Publish(topic, payload, false, qos)
}

// connected is always true: the link exists only while the embedded broker
// it was made for is running.
func (l *inlineLink) connected() bool {
	return true
}

func (l *inlineLink) close() {
	l.mu.Lock()
	for id, filter := range l.subscriptions {
		_ = l.server.Unsubscribe(filter, id)
		delete(l.subscriptions, id)
	}
	l.mu.Unlock()
	l.closeOnce.Do(func() { close(l.done) })
}
