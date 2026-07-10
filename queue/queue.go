// Package queue turns a durable, possibly distributed message queue into a
// gojob source with at-least-once delivery and acknowledgement, so a job can
// survive crashes and be shared across processes.
//
// Consume returns a stream of gojob.Result — the same type gojob.Process
// returns — so the rest of the pipeline (WithStats, ReportEvery, WriteJSONL,
// web.Serve, prom.Push) plugs in unchanged.
package queue

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrClosed is returned by Publish on a sealed or closed queue.
var ErrClosed = errors.New("queue: closed")

// Handlers are the finalization callbacks a backend attaches to a Message.
// Any of them may be nil.
type Handlers struct {
	Ack        func(context.Context) error
	Nack       func(context.Context) error
	DeadLetter func(context.Context) error
}

// Message is a payload delivered from a Queue together with the handle needed to
// acknowledge it. Exactly one of Ack, Nack, or DeadLetter should be called; the
// backend is expected to make repeated calls a no-op.
type Message[T any] struct {
	Payload    T
	deliveries int
	h          Handlers
}

// NewMessage builds a Message for a backend to deliver. deliveries is how many
// times this payload has been delivered so far, counting this delivery (>= 1).
func NewMessage[T any](payload T, deliveries int, h Handlers) *Message[T] {
	return &Message[T]{Payload: payload, deliveries: deliveries, h: h}
}

// Deliveries reports how many times this payload has been delivered, including
// the current delivery.
func (m *Message[T]) Deliveries() int { return m.deliveries }

// Ack marks the message as successfully processed and removes it from the queue.
func (m *Message[T]) Ack(ctx context.Context) error {
	if m.h.Ack == nil {
		return nil
	}
	return m.h.Ack(ctx)
}

// Nack returns the message to the queue for later redelivery.
func (m *Message[T]) Nack(ctx context.Context) error {
	if m.h.Nack == nil {
		return nil
	}
	return m.h.Nack(ctx)
}

// DeadLetter removes the message from the main queue, diverting it to a
// dead-letter store if the backend has one; otherwise it falls back to Ack.
func (m *Message[T]) DeadLetter(ctx context.Context) error {
	if m.h.DeadLetter == nil {
		return m.Ack(ctx)
	}
	return m.h.DeadLetter(ctx)
}

// Queue is a durable source and sink of payloads of type T.
type Queue[T any] interface {
	// Publish enqueues a payload.
	Publish(ctx context.Context, payload T) error
	// Receive streams messages until the queue is drained (for batch queues that
	// have been sealed), ctx is cancelled, or the queue is closed. Backends
	// handle leasing and redelivery of un-acknowledged messages.
	Receive(ctx context.Context) (<-chan *Message[T], error)
	// Close releases the queue's resources.
	Close() error
}

// Codec serializes payloads for durable backends. JSONCodec is the default.
type Codec[T any] interface {
	Marshal(T) ([]byte, error)
	Unmarshal([]byte) (T, error)
}

// JSONCodec encodes payloads as JSON.
type JSONCodec[T any] struct{}

// Marshal implements Codec.
func (JSONCodec[T]) Marshal(v T) ([]byte, error) { return json.Marshal(v) }

// Unmarshal implements Codec.
func (JSONCodec[T]) Unmarshal(b []byte) (T, error) {
	var v T
	err := json.Unmarshal(b, &v)
	return v, err
}
