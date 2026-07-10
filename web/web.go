// Package web serves a small, self-contained live dashboard for a gojob run.
// It is a plain gojob.Stats consumer — the core library has no web dependency;
// only importing this package pulls one in (and even here it is stdlib-only).
//
//	results, stats := gojob.WithStats(ctx, results)
//	go web.Serve(ctx, stats, ":8080")
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/WangYihang/gojob"
)

type config struct {
	interval time.Duration
	title    string
}

// Option configures the dashboard.
type Option func(*config)

// WithInterval sets how often the dashboard refreshes (default 1s).
func WithInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithTitle sets the dashboard heading (default "gojob").
func WithTitle(title string) Option {
	return func(c *config) {
		c.title = title
	}
}

// Handler returns the dashboard as an http.Handler: "/" serves the page and
// "/events" streams progress as Server-Sent Events. It observes stats and stops
// streaming a client when that client disconnects, when the job finishes, or
// when ctx is cancelled. Use it directly to mount the dashboard on your own mux.
func Handler(ctx context.Context, stats *gojob.Stats, opts ...Option) http.Handler {
	cfg := config{interval: time.Second, title: "gojob"}
	for _, o := range opts {
		o(&cfg)
	}
	page := strings.ReplaceAll(indexHTML, "{{TITLE}}", html.EscapeString(cfg.title))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ticker := time.NewTicker(cfg.interval)
		defer ticker.Stop()

		writeEvent(w, flusher, stats.Snapshot(), false) // send current state immediately
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ctx.Done():
				return
			case <-stats.Done():
				writeEvent(w, flusher, stats.Snapshot(), true) // final snapshot
				return
			case <-ticker.C:
				writeEvent(w, flusher, stats.Snapshot(), false)
			}
		}
	})
	return mux
}

// Serve runs the dashboard HTTP server at addr until ctx is cancelled, then
// shuts it down. It blocks, so run it in its own goroutine.
func Serve(ctx context.Context, stats *gojob.Stats, addr string, opts ...Option) error {
	srv := &http.Server{Addr: addr, Handler: Handler(ctx, stats, opts...)}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

type update struct {
	Total     int64 `json:"total"`
	Done      int64 `json:"done"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
	ElapsedMs int64 `json:"elapsed_ms"`
	Finished  bool  `json:"finished"`
}

func writeEvent(w http.ResponseWriter, f http.Flusher, snap gojob.Snapshot, finished bool) {
	b, err := json.Marshal(update{
		Total:     snap.Total,
		Done:      snap.Done,
		Succeeded: snap.Succeeded,
		Failed:    snap.Failed,
		ElapsedMs: snap.Elapsed.Milliseconds(),
		Finished:  finished,
	})
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", b)
	f.Flush()
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{TITLE}}</title>
<style>
  :root{--bg:#f6f7f9;--card:#fff;--fg:#1a1d21;--muted:#6b7280;--line:#e5e7eb;--ok:#16a34a;--fail:#dc2626;--accent:#2563eb}
  @media (prefers-color-scheme:dark){:root{--bg:#0d1117;--card:#161b22;--fg:#e6edf3;--muted:#8b949e;--line:#30363d;--ok:#3fb950;--fail:#f85149;--accent:#58a6ff}}
  *{box-sizing:border-box}
  body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--bg);color:var(--fg);font:15px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
  .card{width:min(560px,92vw);background:var(--card);border:1px solid var(--line);border-radius:16px;padding:28px;box-shadow:0 1px 3px rgba(0,0,0,.06)}
  .head{display:flex;align-items:center;justify-content:space-between;margin-bottom:18px}
  h1{font-size:18px;margin:0;font-weight:600}
  .badge{font-size:12px;font-weight:600;padding:4px 11px;border-radius:999px;border:1px solid var(--accent);color:var(--accent)}
  .badge.done{border-color:var(--ok);color:var(--ok)}
  .pct{font-size:42px;font-weight:700;line-height:1}
  .pct small{font-size:16px;color:var(--muted);font-weight:500;margin-left:2px}
  .bar{height:10px;background:var(--line);border-radius:999px;overflow:hidden;margin:14px 0 22px}
  .bar>i{display:block;height:100%;width:0;background:var(--accent);border-radius:999px;transition:width .4s ease}
  .grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
  .tile{background:var(--bg);border:1px solid var(--line);border-radius:10px;padding:12px}
  .tile .k{font-size:11px;text-transform:uppercase;letter-spacing:.04em;color:var(--muted)}
  .tile .v{font-size:20px;font-weight:600;margin-top:3px;font-variant-numeric:tabular-nums}
  .v.ok{color:var(--ok)}.v.fail{color:var(--fail)}
  @media (max-width:480px){.grid{grid-template-columns:repeat(2,1fr)}}
</style>
</head>
<body>
  <div class="card">
    <div class="head"><h1>{{TITLE}}</h1><span id="status" class="badge">Connecting…</span></div>
    <div class="pct"><span id="pct">0</span><small>%</small></div>
    <div class="bar"><i id="fill"></i></div>
    <div class="grid">
      <div class="tile"><div class="k">Done</div><div class="v" id="done">0</div></div>
      <div class="tile"><div class="k">Succeeded</div><div class="v ok" id="ok">0</div></div>
      <div class="tile"><div class="k">Failed</div><div class="v fail" id="fail">0</div></div>
      <div class="tile"><div class="k">Rate</div><div class="v" id="rate">0/s</div></div>
    </div>
  </div>
<script>
  var $=function(id){return document.getElementById(id)};
  var nf=new Intl.NumberFormat();
  var es=new EventSource('/events');
  es.onmessage=function(e){
    var d=JSON.parse(e.data);
    var total=d.total,done=d.done;
    var pct=total>0?Math.min(100,Math.round(done/total*1000)/10):(d.finished?100:null);
    $('pct').textContent=(pct===null?'—':pct);
    $('fill').style.width=(pct===null?0:pct)+'%';
    $('done').textContent=total>0?nf.format(done)+' / '+nf.format(total):nf.format(done);
    $('ok').textContent=nf.format(d.succeeded);
    $('fail').textContent=nf.format(d.failed);
    var rate=d.elapsed_ms>0?done/(d.elapsed_ms/1000):0;
    $('rate').textContent=(rate>=100?Math.round(rate):Math.round(rate*10)/10)+'/s';
    var st=$('status');
    if(d.finished){st.textContent='Completed';st.className='badge done'}
    else{st.textContent='Running';st.className='badge'}
  };
  es.onerror=function(){var st=$('status');if(st.textContent!=='Completed'){st.textContent='Disconnected';st.className='badge'}};
</script>
</body>
</html>`
