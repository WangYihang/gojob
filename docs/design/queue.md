# Design: `gojob/queue` — durable, resumable, distributable jobs

## Goal

Let a gojob run pull its work from a **durable message queue** instead of an
in-memory channel, so that:

- **Crash-resume** — work in flight when a process dies is redelivered and
  finished by a restarted (or another) consumer.
- **Distribution** — several processes consume the same queue; each item goes to
  exactly one consumer (the queue load-balances, so `Shard` is not needed).

…without disturbing the rest of the pipeline. `Consume` returns
`<-chan gojob.Result[Out]`, the same type `Process` returns, so `WithStats`,
`ReportEvery`, `WriteJSONL`, `web.Serve`, and `prom.Push` all keep working.

```go
q, _ := fileq.Open[Job]("jobs.q")
results, _ := queue.Consume(ctx, q, handle,
    queue.WithWorkers(32),
    queue.WithRetry(4, gojob.ExpBackoff(100*time.Millisecond, 10*time.Second)),
    queue.WithTimeout(16*time.Second),
    queue.WithMaxDeliveries(5),
)
results, stats := gojob.WithStats(ctx, results)
go web.Serve(ctx, stats, ":8080")
gojob.WriteJSONL(ctx, out, results)
```

## Interfaces

```go
type Message[T any] struct { Payload T /* + deliveries, ack/nack/dlq handlers */ }
func (m *Message[T]) Ack(ctx) error         // done — remove from queue
func (m *Message[T]) Nack(ctx) error        // failed — return for redelivery
func (m *Message[T]) DeadLetter(ctx) error  // give up — divert to dead-letter store
func (m *Message[T]) Deliveries() int       // times delivered so far (>= 1)

type Queue[T any] interface {
    Publish(ctx, payload T) error
    Receive(ctx) (<-chan *Message[T], error)
    Close() error
}

type Codec[T any] interface { Marshal(T) ([]byte, error); Unmarshal([]byte) (T, error) }

func Consume[In, Out any](ctx, q Queue[In], fn func(ctx, In) (Out, error), opts ...Option) (<-chan gojob.Result[Out], error)
func Fill[T any](ctx, q Queue[T], in <-chan T) (int, error)
```

## The one hard part: acknowledgement correlation

To ack we must carry the message handle from the input, through `Process`, to the
result. But `Result[Out]` only carries `Out`, and `WithTimeout` returns the *zero*
`Out` on timeout — losing the handle exactly when a task hangs.

**Solution:** apply the per-attempt timeout *inside* the work function (not via
`gojob.WithTimeout`), and bundle the handle with the output. Then the outcome
always carries the message, even on timeout, and the ack step runs on the final
`Result`:

```go
work := func(ctx, m *Message[In]) (outcome, error) {
    out, err := runWithTimeout(ctx, cfg.timeout, func(ctx) (Out, error) { return fn(ctx, m.Payload) })
    return outcome{msg: m, out: out}, err          // msg always set
}
processed := gojob.Process(ctx, msgs, work, WithWorkers(n), WithRetry(r, backoff)) // NOT WithTimeout
for res := range processed {
    m := res.Value.msg
    switch {
    case res.Err == nil:                             m.Ack(ctx)         // emit success
    case cfg.maxDeliveries > 0 && m.Deliveries() >= cfg.maxDeliveries: m.DeadLetter(ctx) // emit final error
    default:                                          m.Nack(ctx); continue // silent — redelivered later
    }
    emit gojob.Result[Out]{...}
}
```

One `Result` is emitted per **finalized** message (ack or dead-letter);
redeliveries are silent, so `stats.Done` counts jobs, not attempts.

## Delivery semantics

- **At-least-once.** Ack happens only after success; a crash before ack causes
  redelivery, so `fn` must be **idempotent**.
- **Two levels of retry.** In-memory (`WithRetry`, fast, lost on crash) for
  transient blips; nack → redelivery (crash-safe, slower) for the rest.
- **Visibility lease.** A received message is invisible to other consumers until
  its lease expires; if it isn't acked in time it is redelivered — this is the
  crash-resume mechanism.
- **Dead-letter.** After `WithMaxDeliveries` deliveries a message is diverted to
  a dead-letter store instead of looping forever (poison-message defense; a
  delivery whose consumer crashed counts, on purpose).

## Backends & dependency isolation

`Queue[T]` is backend-agnostic; durable backends use a `Codec[T]` (JSON default).

| Backend | Package | Deps | Use |
| --- | --- | --- | --- |
| in-memory | `queue/memq` | none | tests / single-process batch |
| file directory | `queue/fileq` | none (stdlib) | single-machine durable + crash-resume |
| Redis | `queue/redisq` | go-redis | cross-machine distribution *(phase 2)* |

Core `queue`, `memq`, and `fileq` are **stdlib-only**; `redisq` goes in its own
module so the Redis dependency never reaches the core — the same isolation used
for `prom` and `web`.

### `fileq` on disk

```
<dir>/pending/<seq>.<deliveries>.<lease>.json     lease 0 = waiting
<dir>/processing/<seq>.<deliveries>.<lease>.json  lease = deadline (unix nano)
<dir>/dead/<seq>.<deliveries>.json
<dir>/sealed                                       producer is done (enables drain)
```

- **Claim** = atomic `rename(pending → processing)` — the rename *is* the lock,
  so two consumers never take the same item.
- **Ack** deletes; **Nack** renames back to `pending`; **DeadLetter** renames to
  `dead`; a **reaper** renames leases that have expired back to `pending`.
- **Drain** (batch): `Receive` closes when `sealed` and both `pending` and
  `processing` are empty. On restart, orphaned `processing` files (expired lease)
  are reclaimed, so a killed consumer resumes.

## Non-goals & trade-offs

- **No ordering guarantee** (like `Process`, results are unordered).
- **No exactly-once** — impossible to guarantee across crashes; at-least-once +
  idempotency instead.
- **Backpressure preserved** — `Consume` pulls only as fast as workers free up;
  the whole queue is never loaded into memory.

## Phasing

1. **MVP** — `queue` (interface, `Consume`, `Codec`, `Fill`) + `memq` + `fileq` +
   `examples/queue` + tests (all stdlib).
2. **Distribution** — `queue/redisq` (separate module) + integration tests.
3. **Optional** — delayed redelivery, priority, batch ack.
