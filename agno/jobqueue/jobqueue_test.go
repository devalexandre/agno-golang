package jobqueue

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func sqliteStore(t *testing.T) *SQLStore {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "q.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	st, err := NewSQLStore(db, SQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func allStores(t *testing.T) map[string]Store {
	m := map[string]Store{"memory": NewMemoryStore(), "sqlite": sqliteStore(t)}
	// Optional Postgres contract run: AGNO_TEST_POSTGRES_DSN=postgres://...
	if dsn := os.Getenv("AGNO_TEST_POSTGRES_DSN"); dsn != "" {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		table := "agno_jobs_test_" + NewID()[:8]
		st, _ := NewSQLStore(db, Postgres, table)
		if err := st.Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.Exec("DROP TABLE " + table) })
		m["postgres"] = st
	}
	return m
}

func newJob(id, session string, serialize bool, created int64, maxAttempts int) *Job {
	return &Job{ID: id, JobType: "run", ComponentType: "agent", ComponentID: "a", SessionID: session,
		SerializeSession: serialize, Status: StatusQueued, MaxAttempts: maxAttempts, AvailableAt: created,
		CreatedAt: created, Payload: map[string]any{"input": id}}
}

func TestStoreContract(t *testing.T) {
	for name, st := range allStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().Unix()
			key := "k1"
			j1 := newJob("j1", "s1", true, now-10, 2)
			j1.IdempotencyKey = &key
			res, err := st.Enqueue(ctx, j1, 0)
			if err != nil || !res.Accepted || res.Job.SessionSeq != 1 {
				t.Fatalf("enqueue %v %+v", err, res)
			}
			dup := newJob("j1b", "s1", false, now, 1)
			dup.IdempotencyKey = &key
			if res, _ := st.Enqueue(ctx, dup, 0); res.Accepted || res.Reason != ReasonDuplicate || res.Job.ID != "j1" {
				t.Fatalf("dedup %+v", res)
			}
			if res, _ := st.Enqueue(ctx, newJob("j2", "s1", true, now-9, 1), 0); !res.Accepted || res.Job.SessionSeq != 2 {
				t.Fatalf("seq %+v", res.Job)
			}
			if _, err := st.Enqueue(ctx, newJob("j3", "s2", false, now-8, 1), 0); err != nil {
				t.Fatal(err)
			}
			if res, _ := st.Enqueue(ctx, newJob("j4", "s3", false, now, 1), 3); res.Accepted || res.Reason != ReasonQueueFull {
				t.Fatalf("depth %+v", res)
			}
			if b, _ := st.SessionBlocker(ctx, "j2"); b != "j1" {
				t.Fatalf("blocker %q", b)
			}

			c1, _ := st.Claim(ctx, "w1", time.Minute, nil)
			if c1 == nil || c1.ID != "j1" || c1.Attempt != 1 {
				t.Fatalf("claim1 %+v", c1)
			}
			// j2 is blocked behind running j1; next is j3.
			c2, _ := st.Claim(ctx, "w2", time.Minute, nil)
			if c2 == nil || c2.ID != "j3" {
				t.Fatalf("claim2 %+v", c2)
			}
			if c3, _ := st.Claim(ctx, "w2", time.Minute, nil); c3 != nil {
				t.Fatalf("head-of-line violated: %+v", c3)
			}
			if n, _ := st.Heartbeat(ctx, "w1", []string{"j1", "j3"}); n != 1 {
				t.Fatalf("heartbeat %d", n)
			}
			// Fenced: wrong worker cannot complete.
			if ok, _ := st.Complete(ctx, "j1", "w2", 1, StatusCompleted, nil); ok {
				t.Fatal("fence broken")
			}
			if s, _ := st.RetryOrFail(ctx, "j1", "w1", 1, "boom", 0); s != StatusQueued {
				t.Fatalf("retry %q", s)
			}
			c4, _ := st.Claim(ctx, "w1", time.Minute, nil)
			if c4 == nil || c4.ID != "j1" || c4.Attempt != 2 {
				t.Fatalf("reclaim %+v", c4)
			}
			if s, _ := st.RetryOrFail(ctx, "j1", "w1", 2, "boom", 0); s != StatusFailed {
				t.Fatalf("fail %q", s)
			}
			c5, _ := st.Claim(ctx, "w1", time.Minute, nil)
			if c5 == nil || c5.ID != "j2" {
				t.Fatalf("session unblocked %+v", c5)
			}
			if ok, _ := st.Complete(ctx, "j2", "w1", 1, StatusPaused, nil); !ok {
				t.Fatal("pause")
			}
			outcome, j, _ := st.Continue(ctx, "j2", map[string]any{"approved": true})
			if outcome != ContinueQueued || j.Payload["continue"] == nil || j.MaxAttempts != 2 {
				t.Fatalf("continue %s %+v", outcome, j)
			}
			if o, _, _ := st.Continue(ctx, "j2", nil); o != ContinueAttach {
				t.Fatalf("attach %s", o)
			}
			if ok, _ := st.Cancel(ctx, "j2"); !ok {
				t.Fatal("cancel")
			}
			if ok, _ := st.Requeue(ctx, "j1"); !ok {
				t.Fatal("requeue")
			}
			got, _ := st.Get(ctx, "j1")
			if got.Status != StatusQueued || got.MaxAttempts != 3 {
				t.Fatalf("requeued %+v", got)
			}
			// Stale + exhausted: j3 (max 1) locked "in the past".
			jobs, _ := st.SweepExhausted(ctx, -time.Second, 10)
			if len(jobs) != 1 || jobs[0].ID != "j3" {
				t.Fatalf("sweep %+v", jobs)
			}
			if ok, _ := st.AcquireSweep(ctx, "j3", "sweeper", -time.Second); !ok {
				t.Fatal("acquire sweep")
			}
			if ok, _ := st.SettleSwept(ctx, "j3", "sweeper", StatusFailed, nil); !ok {
				t.Fatal("settle swept")
			}
			stats, _ := st.Stats(ctx)
			if stats.Counts[StatusQueued] != 1 || stats.Counts[StatusFailed] != 1 || stats.Counts[StatusCancelled] != 1 {
				t.Fatalf("stats %+v", stats.Counts)
			}
			list, total, _ := st.List(ctx, ListFilter{Statuses: []string{StatusFailed, StatusCancelled}})
			if total != 2 || len(list) != 2 {
				t.Fatalf("list %d", total)
			}
			if n, _ := st.Cleanup(ctx, -time.Second); n != 2 {
				t.Fatalf("cleanup %d", n)
			}
			if _, err := st.Get(ctx, "j3"); !errors.Is(err, ErrNotFound) {
				t.Fatal("expected not found")
			}
		})
	}
}

func TestStaleReclaim(t *testing.T) {
	for name, st := range allStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_, _ = st.Enqueue(ctx, newJob("r1", "s", false, time.Now().Unix(), 2), 0)
			c, _ := st.Claim(ctx, "dead", time.Minute, nil)
			if c == nil {
				t.Fatal("claim")
			}
			if c2, _ := st.Claim(ctx, "w2", time.Minute, nil); c2 != nil {
				t.Fatal("live lease reclaimed")
			}
			c3, _ := st.Claim(ctx, "w2", -time.Second, nil)
			if c3 == nil || c3.Attempt != 2 || *c3.LockedBy != "w2" {
				t.Fatalf("stale reclaim %+v", c3)
			}
			// The dead worker's late completion is fenced out.
			if ok, _ := st.Complete(ctx, "r1", "dead", 1, StatusCompleted, nil); ok {
				t.Fatal("stale writer not fenced")
			}
		})
	}
}

func TestWorkerPerSessionOrder(t *testing.T) {
	for name, st := range allStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var (
				mu       sync.Mutex
				order    []string
				active   = map[string]int{}
				overlaps int32
			)
			handler := func(ctx context.Context, j *Job) (Outcome, error) {
				mu.Lock()
				active[j.SessionID]++
				if active[j.SessionID] > 1 {
					atomic.AddInt32(&overlaps, 1)
				}
				mu.Unlock()
				time.Sleep(15 * time.Millisecond)
				mu.Lock()
				active[j.SessionID]--
				order = append(order, j.Payload["n"].(string))
				mu.Unlock()
				return Outcome{}, nil
			}
			w := NewWorker(st, handler, Config{MaxConcurrency: 8, PollInterval: 5 * time.Millisecond})
			for _, n := range []string{"a1", "a2", "a3"} {
				res, err := w.Enqueue(ctx, EnqueueOptions{SessionID: "A", SerializeSession: true, Payload: map[string]any{"n": n}})
				if err != nil || !res.Accepted {
					t.Fatal(err)
				}
			}
			_, _ = w.Enqueue(ctx, EnqueueOptions{SessionID: "B", Payload: map[string]any{"n": "b1"}})
			w.Start(ctx)
			deadline := time.Now().Add(5 * time.Second)
			for {
				st2, _ := st.Stats(ctx)
				if st2.Counts[StatusCompleted] == 4 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("timeout: %+v", st2.Counts)
				}
				time.Sleep(5 * time.Millisecond)
			}
			_ = w.Stop(ctx)
			if overlaps != 0 {
				t.Fatal("two turns of one session ran concurrently")
			}
			var seq []string
			for _, o := range order {
				if o != "b1" {
					seq = append(seq, o)
				}
			}
			if len(seq) != 3 || seq[0] != "a1" || seq[1] != "a2" || seq[2] != "a3" {
				t.Fatalf("order %v", order)
			}
		})
	}
}

func TestWorkerRetryCancelPermanent(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	var calls int32
	started := make(chan string, 4)
	handler := func(ctx context.Context, j *Job) (Outcome, error) {
		switch j.Payload["kind"] {
		case "flaky":
			if atomic.AddInt32(&calls, 1) == 1 {
				return Outcome{}, errors.New("transient")
			}
			return Outcome{}, nil
		case "bad":
			return Outcome{}, Permanent(errors.New("bad input"))
		case "slow":
			started <- j.ID
			<-ctx.Done()
			return Outcome{}, ctx.Err()
		case "pause":
			if j.Payload["continue"] != nil {
				return Outcome{}, nil
			}
			return Outcome{Status: StatusPaused}, nil
		}
		return Outcome{}, nil
	}
	w := NewWorker(st, handler, Config{MaxAttempts: 3, RetryDelay: time.Millisecond, PollInterval: 5 * time.Millisecond})
	w.Start(ctx)
	defer w.Stop(ctx) //nolint:errcheck
	flaky, _ := w.Enqueue(ctx, EnqueueOptions{SessionID: "s", Payload: map[string]any{"kind": "flaky"}})
	bad, _ := w.Enqueue(ctx, EnqueueOptions{SessionID: "s", Payload: map[string]any{"kind": "bad"}})
	slow, _ := w.Enqueue(ctx, EnqueueOptions{SessionID: "s", Payload: map[string]any{"kind": "slow"}})
	pause, _ := w.Enqueue(ctx, EnqueueOptions{SessionID: "s", Payload: map[string]any{"kind": "pause"}})
	<-started
	if ok, _ := w.Cancel(ctx, slow.Job.ID); !ok {
		t.Fatal("cancel running")
	}
	wait := func(id, status string) *Job {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			j, _ := st.Get(ctx, id)
			if j.Status == status {
				return j
			}
			time.Sleep(5 * time.Millisecond)
		}
		j, _ := st.Get(ctx, id)
		t.Fatalf("job %s: want %s got %+v", id, status, j)
		return nil
	}
	if j := wait(flaky.Job.ID, StatusCompleted); j.Attempt != 2 {
		t.Fatalf("flaky attempts %d", j.Attempt)
	}
	if j := wait(bad.Job.ID, StatusFailed); j.Attempt != 1 {
		t.Fatalf("permanent retried %d", j.Attempt)
	}
	wait(slow.Job.ID, StatusCancelled)
	wait(pause.Job.ID, StatusPaused)
	if o, _, _ := w.Continue(ctx, pause.Job.ID, map[string]any{"ok": true}); o != ContinueQueued {
		t.Fatal(o)
	}
	wait(pause.Job.ID, StatusCompleted)
}

func TestWorkerSweepsExhausted(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	_, _ = st.Enqueue(ctx, newJob("z", "s", false, time.Now().Unix(), 1), 0)
	if c, _ := st.Claim(ctx, "dead", time.Minute, nil); c == nil {
		t.Fatal("claim")
	}
	w := NewWorker(st, func(context.Context, *Job) (Outcome, error) { return Outcome{}, nil }, Config{LockGrace: time.Second})
	w.Config.LockGrace = -time.Second // treat every lease as stale
	w.Sweep(ctx)
	j, _ := st.Get(ctx, "z")
	if j.Status != StatusFailed || j.Error == nil {
		t.Fatalf("%+v", j)
	}
}

func TestRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := NewMemoryStore()
	ctx := context.Background()
	_, _ = st.Enqueue(ctx, newJob("x", "s", false, time.Now().Unix(), 1), 0)
	r := gin.New()
	RegisterRoutes(r.Group("/queue"), st, nil)
	for path, want := range map[string]int{"/queue/jobs?status=queued": 200, "/queue/jobs/x": 200, "/queue/jobs/nope": 404, "/queue/stats": 200, "/queue/jobs?status=bogus": 400} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/queue/jobs/x/cancel", nil))
	if w.Code != 200 {
		t.Fatalf("cancel %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/queue/jobs/x/requeue", nil))
	if w.Code != 200 {
		t.Fatalf("requeue %d", w.Code)
	}
}
