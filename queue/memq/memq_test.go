package memq_test

import (
	"context"
	"testing"

	"github.com/WangYihang/gojob/queue"
	"github.com/WangYihang/gojob/queue/memq"
)

func TestPublishReceiveAck(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	_ = q.Publish(ctx, 7)
	q.Seal()

	ch, _ := q.Receive(ctx)
	m := <-ch
	if m.Payload != 7 {
		t.Errorf("payload = %d, want 7", m.Payload)
	}
	if m.Deliveries() != 1 {
		t.Errorf("deliveries = %d, want 1", m.Deliveries())
	}
	_ = m.Ack(ctx)
	if _, ok := <-ch; ok {
		t.Error("expected Receive channel to close after the queue drains")
	}
}

func TestNackRedelivers(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	_ = q.Publish(ctx, 1)
	q.Seal()

	ch, _ := q.Receive(ctx)
	m1 := <-ch
	if m1.Deliveries() != 1 {
		t.Errorf("first delivery count = %d, want 1", m1.Deliveries())
	}
	_ = m1.Nack(ctx)
	m2 := <-ch
	if m2.Deliveries() != 2 {
		t.Errorf("redelivery count = %d, want 2", m2.Deliveries())
	}
	_ = m2.Ack(ctx)
	if _, ok := <-ch; ok {
		t.Error("expected channel to close after ack")
	}
}

func TestDeadLetter(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	_ = q.Publish(ctx, 9)
	q.Seal()

	ch, _ := q.Receive(ctx)
	m := <-ch
	_ = m.DeadLetter(ctx)
	if _, ok := <-ch; ok {
		t.Error("expected channel to close after dead-letter")
	}
	if d := q.Dead(); len(d) != 1 || d[0] != 9 {
		t.Errorf("dead = %v, want [9]", d)
	}
}

func TestPublishAfterSeal(t *testing.T) {
	q := memq.New[int]()
	q.Seal()
	if err := q.Publish(context.Background(), 1); err != queue.ErrClosed {
		t.Errorf("Publish after Seal = %v, want ErrClosed", err)
	}
}
