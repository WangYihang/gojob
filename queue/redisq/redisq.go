// Package redisq is a Redis-backed queue.Queue for cross-machine distribution.
// Several processes on different hosts may Consume the same queue: Redis hands
// each message to exactly one consumer, and a message that is not acknowledged
// before its visibility lease expires is reclaimed and redelivered — so a
// crashed consumer's work is finished elsewhere.
//
// It is kept out of the core module's compile path: importing gojob does not
// pull in Redis; only importing this package does.
package redisq

import (
	"context"
	"encoding/base64"
	"strconv"
	"sync"
	"time"

	"github.com/WangYihang/gojob/queue"
	"github.com/redis/go-redis/v9"
)

type config struct {
	lease  time.Duration
	poll   time.Duration
	prefix string
}

// Option configures a Queue.
type Option func(*config)

// WithLease sets how long a claimed message stays invisible before it is
// considered abandoned and redelivered (default 30s).
func WithLease(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.lease = d
		}
	}
}

// WithPollInterval sets how often Receive polls when idle (default 200ms).
func WithPollInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.poll = d
		}
	}
}

// WithPrefix namespaces the queue's Redis keys (default "gojob").
func WithPrefix(p string) Option {
	return func(c *config) {
		if p != "" {
			c.prefix = p
		}
	}
}

type keys struct {
	seq, pending, processing, leases, payloads, deliveries, dead, sealed string
}

// Queue is a Redis-backed queue.Queue[T]. The caller owns the redis client.
type Queue[T any] struct {
	rdb   redis.UniversalClient
	codec queue.Codec[T]
	lease time.Duration
	poll  time.Duration
	k     keys
}

// New creates a queue over rdb. Payloads are encoded as JSON.
func New[T any](rdb redis.UniversalClient, opts ...Option) *Queue[T] {
	cfg := config{lease: 30 * time.Second, poll: 200 * time.Millisecond, prefix: "gojob"}
	for _, o := range opts {
		o(&cfg)
	}
	p := cfg.prefix
	return &Queue[T]{
		rdb:   rdb,
		codec: queue.JSONCodec[T]{},
		lease: cfg.lease,
		poll:  cfg.poll,
		k: keys{
			seq: p + ":seq", pending: p + ":pending", processing: p + ":processing",
			leases: p + ":leases", payloads: p + ":payloads", deliveries: p + ":deliveries",
			dead: p + ":dead", sealed: p + ":sealed",
		},
	}
}

// Close is a no-op: the caller owns and closes the redis client.
func (q *Queue[T]) Close() error { return nil }

// Publish enqueues a payload.
func (q *Queue[T]) Publish(ctx context.Context, payload T) error {
	b, err := q.codec.Marshal(payload)
	if err != nil {
		return err
	}
	n, err := q.rdb.Incr(ctx, q.k.seq).Result()
	if err != nil {
		return err
	}
	id := strconv.FormatInt(n, 10)
	_, err = q.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, q.k.payloads, id, base64.StdEncoding.EncodeToString(b))
		p.LPush(ctx, q.k.pending, id)
		return nil
	})
	return err
}

// Seal marks the queue complete so Receive drains and closes once pending and
// processing are both empty.
func (q *Queue[T]) Seal(ctx context.Context) error {
	return q.rdb.Set(ctx, q.k.sealed, "1", 0).Err()
}

// DeadCount reports how many messages are in the dead-letter list.
func (q *Queue[T]) DeadCount(ctx context.Context) int {
	n, _ := q.rdb.LLen(ctx, q.k.dead).Result()
	return int(n)
}

// claimScript atomically moves the oldest pending id into processing, bumps its
// delivery count, records a lease, and returns {id, deliveries, payload}.
var claimScript = redis.NewScript(`
local id = redis.call('RPOPLPUSH', KEYS[1], KEYS[2])
if not id then return false end
local d = redis.call('HINCRBY', KEYS[3], id, 1)
redis.call('ZADD', KEYS[4], ARGV[1], id)
local p = redis.call('HGET', KEYS[5], id)
return {id, d, p}
`)

// drainScript reports whether the queue is sealed and both lists are empty,
// reading all three keys in one atomic snapshot so a message mid-nack (moving
// from processing to pending) is never observed as absent from both.
var drainScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
if redis.call('LLEN', KEYS[2]) ~= 0 then return 0 end
if redis.call('LLEN', KEYS[3]) ~= 0 then return 0 end
return 1
`)

// reclaimScript moves ids whose lease has expired from processing back to pending.
var reclaimScript = redis.NewScript(`
local expired = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
for i = 1, #expired do
  local id = expired[i]
  redis.call('LREM', KEYS[2], 1, id)
  redis.call('ZREM', KEYS[1], id)
  redis.call('LPUSH', KEYS[3], id)
end
return #expired
`)

// Receive streams messages, reclaiming expired leases as it goes.
func (q *Queue[T]) Receive(ctx context.Context) (<-chan *queue.Message[T], error) {
	out := make(chan *queue.Message[T])
	go func() {
		defer close(out)
		for {
			if ctx.Err() != nil {
				return
			}
			_ = reclaimScript.Run(ctx, q.rdb,
				[]string{q.k.leases, q.k.processing, q.k.pending},
				time.Now().UnixNano(),
			).Err()

			m, ok, err := q.claim(ctx)
			if err != nil || !ok {
				if err == nil && q.drained(ctx) {
					return
				}
				select {
				case <-time.After(q.poll):
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case out <- m:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (q *Queue[T]) claim(ctx context.Context) (*queue.Message[T], bool, error) {
	deadline := time.Now().Add(q.lease).UnixNano()
	res, err := claimScript.Run(ctx, q.rdb,
		[]string{q.k.pending, q.k.processing, q.k.deliveries, q.k.leases, q.k.payloads},
		deadline,
	).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return nil, false, nil
	}
	id, _ := arr[0].(string)
	deliveries := toInt(arr[1])
	b64, _ := arr[2].(string)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		q.divert(id)
		return nil, false, nil
	}
	payload, err := q.codec.Unmarshal(raw)
	if err != nil {
		q.divert(id)
		return nil, false, nil
	}
	return q.wrap(id, deliveries, payload), true, nil
}

func (q *Queue[T]) wrap(id string, deliveries int, payload T) *queue.Message[T] {
	var once sync.Once
	do := func(fn func() error) error {
		var err error
		once.Do(func() { err = fn() })
		return err
	}
	return queue.NewMessage(payload, deliveries, queue.Handlers{
		Ack:        func(context.Context) error { return do(func() error { return q.ack(id) }) },
		Nack:       func(context.Context) error { return do(func() error { return q.nack(id) }) },
		DeadLetter: func(context.Context) error { return do(func() error { return q.deadLetter(id) }) },
	})
}

// Finalization uses a fresh short-lived context so an ack still lands even when
// the consume context is being cancelled during shutdown.
func (q *Queue[T]) finalize(fn func(context.Context, redis.Pipeliner)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := q.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		fn(ctx, p)
		return nil
	})
	return err
}

func (q *Queue[T]) ack(id string) error {
	return q.finalize(func(ctx context.Context, p redis.Pipeliner) {
		p.LRem(ctx, q.k.processing, 1, id)
		p.ZRem(ctx, q.k.leases, id)
		p.HDel(ctx, q.k.payloads, id)
		p.HDel(ctx, q.k.deliveries, id)
	})
}

func (q *Queue[T]) nack(id string) error {
	return q.finalize(func(ctx context.Context, p redis.Pipeliner) {
		p.LRem(ctx, q.k.processing, 1, id)
		p.ZRem(ctx, q.k.leases, id)
		p.LPush(ctx, q.k.pending, id) // delivery count retained in the hash
	})
}

func (q *Queue[T]) deadLetter(id string) error {
	return q.finalize(func(ctx context.Context, p redis.Pipeliner) {
		p.LRem(ctx, q.k.processing, 1, id)
		p.ZRem(ctx, q.k.leases, id)
		p.LPush(ctx, q.k.dead, id)
		p.HDel(ctx, q.k.deliveries, id)
	})
}

func (q *Queue[T]) divert(id string) {
	_ = q.finalize(func(ctx context.Context, p redis.Pipeliner) {
		p.LRem(ctx, q.k.processing, 1, id)
		p.ZRem(ctx, q.k.leases, id)
		p.LPush(ctx, q.k.dead, id)
	})
}

func (q *Queue[T]) drained(ctx context.Context) bool {
	n, err := drainScript.Run(ctx, q.rdb, []string{q.k.sealed, q.k.pending, q.k.processing}).Int()
	return err == nil && n == 1
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
