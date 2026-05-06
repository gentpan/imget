package imgpipe

import (
	"context"
	"log/slog"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"imget/internal/db"
)

// profileBatcher accumulates per-(w,h,type,keyword) counter increments in
// memory and flushes them to SQLite on a fixed cadence. Without this, every
// HTTP request would issue an UPSERT — fine at low QPS, but at 1k+ RPS the
// SQLite write lock becomes the bottleneck.
//
// First-time profile registration still hits the DB synchronously (so detail
// pages can read view/download counters back). Subsequent visits to the same
// profile only touch in-memory counters until the next flush tick.
type profileBatcher struct {
	db            *db.DB
	log           *slog.Logger
	flushInterval time.Duration

	mu      sync.Mutex
	pending map[string]*pendingProfile

	// seen profiles already inserted this process lifetime. On hit, we skip
	// the synchronous DB UPSERT and just bump the in-memory counter.
	seen *lru.Cache[string, struct{}]

	// stop signals the background flusher to drain + exit.
	stop chan struct{}
	done chan struct{}
}

type pendingProfile struct {
	requestCount    int64
	viewCount       int64
	downloadCount   int64
	lastRequestedAt int64
}

func newProfileBatcher(sqlDB *db.DB, log *slog.Logger, flushInterval time.Duration) *profileBatcher {
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	seen, _ := lru.New[string, struct{}](20_000)
	return &profileBatcher{
		db:            sqlDB,
		log:           log,
		flushInterval: flushInterval,
		pending:       make(map[string]*pendingProfile),
		seen:          seen,
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
}

// Run drives the background flush loop. Blocks until Close is called.
func (b *profileBatcher) Run() {
	defer close(b.done)
	t := time.NewTicker(b.flushInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			b.flush()
		case <-b.stop:
			b.flush() // drain final batch before exit
			return
		}
	}
}

// Close stops the flusher and waits for the final flush to complete.
func (b *profileBatcher) Close() {
	close(b.stop)
	<-b.done
}

// Register increments request_count for a profile. On first sight (LRU miss)
// it does a synchronous DB UPSERT so subsequent reads find the row.
func (b *profileBatcher) Register(ctx context.Context, w, h int, typ, keyword string) (string, error) {
	key := db.ProfileKey(w, h, typ, keyword)
	now := time.Now().Unix()

	if _, ok := b.seen.Get(key); ok {
		// Hot path — pure in-memory.
		b.mu.Lock()
		p := b.pending[key]
		if p == nil {
			p = &pendingProfile{}
			b.pending[key] = p
		}
		p.requestCount++
		if now > p.lastRequestedAt {
			p.lastRequestedAt = now
		}
		b.mu.Unlock()
		return key, nil
	}

	// Cold path — first time we see this profile this process. Do the
	// existing synchronous UPSERT so reads can find the row immediately.
	if _, err := b.db.RegisterProfile(ctx, w, h, typ, keyword); err != nil {
		return "", err
	}
	b.seen.Add(key, struct{}{})
	return key, nil
}

// IncView / IncDownload bump in-memory counters. They never block on the DB.
// Used by future __track endpoint instrumentation.
func (b *profileBatcher) IncView(profileKey string)     { b.inc(profileKey, 0, 1, 0) }
func (b *profileBatcher) IncDownload(profileKey string) { b.inc(profileKey, 0, 0, 1) }

func (b *profileBatcher) inc(key string, req, view, dl int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.pending[key]
	if p == nil {
		p = &pendingProfile{}
		b.pending[key] = p
	}
	p.requestCount += req
	p.viewCount += view
	p.downloadCount += dl
}

// flush moves the entire pending map into one transactional batch update.
func (b *profileBatcher) flush() {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return
	}
	pending := b.pending
	b.pending = make(map[string]*pendingProfile, len(pending))
	b.mu.Unlock()

	deltas := make([]db.ProfileMetricsDelta, 0, len(pending))
	for k, v := range pending {
		deltas = append(deltas, db.ProfileMetricsDelta{
			ProfileKey:       k,
			AddRequestCount:  v.requestCount,
			AddViewCount:     v.viewCount,
			AddDownloadCount: v.downloadCount,
			LastRequestedAt:  v.lastRequestedAt,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.db.BatchAddProfileMetrics(ctx, deltas); err != nil {
		b.log.Warn("profile batch flush failed", "err", err, "rows", len(deltas))
		// Push deltas back so we don't lose them.
		b.mu.Lock()
		for k, v := range pending {
			cur := b.pending[k]
			if cur == nil {
				cur = &pendingProfile{}
				b.pending[k] = cur
			}
			cur.requestCount += v.requestCount
			cur.viewCount += v.viewCount
			cur.downloadCount += v.downloadCount
			if v.lastRequestedAt > cur.lastRequestedAt {
				cur.lastRequestedAt = v.lastRequestedAt
			}
		}
		b.mu.Unlock()
		return
	}
}
