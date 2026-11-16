package scheduler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func TestCronNext(t *testing.T) {
	cases := []struct {
		expr, from, want string
	}{
		{"*/15 * * * *", "2026-01-01T10:07:30Z", "2026-01-01T10:15:00Z"},
		{"0 9 * * MON-FRI", "2026-01-02T09:00:00Z", "2026-01-05T09:00:00Z"}, // Fri 09:00 -> Mon
		{"30 2 1 * *", "2026-01-15T00:00:00Z", "2026-02-01T02:30:00Z"},
		{"@hourly", "2026-01-01T10:59:00Z", "2026-01-01T11:00:00Z"},
		{"0 0 29 2 *", "2026-03-01T00:00:00Z", "2028-02-29T00:00:00Z"},
		{"0 12 * * 7", "2026-01-01T00:00:00Z", "2026-01-04T12:00:00Z"},   // Sunday
		{"0 0 1,15 * 1", "2026-01-02T00:00:00Z", "2026-01-05T00:00:00Z"}, // dom OR dow
	}
	for _, c := range cases {
		cr, err := ParseCron(c.expr)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		from, _ := time.Parse(time.RFC3339, c.from)
		got := cr.Next(from).UTC().Format(time.RFC3339)
		if got != c.want {
			t.Errorf("%s from %s: got %s want %s", c.expr, c.from, got, c.want)
		}
	}
	for _, bad := range []string{"* * *", "60 * * * *", "* 24 * * *", "*/0 * * * *", "a b c d e"} {
		if ValidateCronExpr(bad) {
			t.Errorf("expected %q invalid", bad)
		}
	}
	if !ValidateCronExpr("") {
		t.Error("empty cron is on-demand and valid")
	}
}

func TestCronTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	cr, _ := ParseCron("0 9 * * *")
	from := time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC).In(loc) // 10:00 local
	next := cr.Next(from)
	if next.In(loc).Hour() != 9 || next.UTC().Hour() != 12 {
		t.Fatalf("unexpected %v", next)
	}
	if !ValidateTimezone("America/New_York") || ValidateTimezone("Mars/Base") {
		t.Fatal("timezone validation")
	}
}

func openSQLite(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func stores(t *testing.T) map[string]Store {
	sqlStore, err := NewSQLStore(openSQLite(t), SQLite, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlStore.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	m := map[string]Store{"memory": NewMemoryStore(), "sqlite": sqlStore}
	// Optional Postgres run: AGNO_TEST_POSTGRES_DSN=postgres://...
	if dsn := os.Getenv("AGNO_TEST_POSTGRES_DSN"); dsn != "" {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		sfx := NewID()[:8]
		pg, _ := NewSQLStore(db, Postgres, &SQLStoreOptions{SchedulesTable: "sched_test_" + sfx, RunsTable: "sched_runs_test_" + sfx})
		if err := pg.Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.Exec("DROP TABLE sched_test_" + sfx + ", sched_runs_test_" + sfx) })
		m["postgres"] = pg
	}
	return m
}

func TestStoreContract(t *testing.T) {
	for name, st := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			m := NewManager(st)
			owner := "u1"
			s, err := m.Create(ctx, CreateRequest{Name: "nightly", CronExpr: "0 3 * * *", Endpoint: "/agents/a1/runs",
				Payload: map[string]any{"message": "hi"}, UserID: &owner})
			if err != nil {
				t.Fatal(err)
			}
			if s.NextRunAt == nil || strOr(s.TargetType) != "agent" || strOr(s.TargetID) != "a1" {
				t.Fatalf("bad schedule %+v", s)
			}
			if _, err := m.Create(ctx, CreateRequest{Name: "nightly", Endpoint: "/x", UserID: &owner}); !errors.Is(err, ErrDuplicateName) {
				t.Fatalf("want duplicate, got %v", err)
			}
			if _, err := m.Create(ctx, CreateRequest{Name: "bad", CronExpr: "nope", Endpoint: "/x"}); !IsValidation(err) {
				t.Fatalf("want validation, got %v", err)
			}
			other := "u2"
			if _, err := m.Get(ctx, s.ID, &other); !errors.Is(err, ErrNotFound) {
				t.Fatal("owner scoping")
			}
			got, err := m.Get(ctx, s.ID, &owner)
			if err != nil || got.Payload["message"] != "hi" {
				t.Fatalf("get: %v %+v", err, got)
			}
			list, total, err := m.List(ctx, ListFilter{UserID: &owner})
			if err != nil || total != 1 || len(list) != 1 {
				t.Fatalf("list %v %d", err, total)
			}
			d, err := m.Disable(ctx, s.ID, nil)
			if err != nil || d.Enabled || d.NextRunAt != nil {
				t.Fatalf("disable %v %+v", err, d)
			}
			e, err := m.Enable(ctx, s.ID, nil)
			if err != nil || !e.Enabled || e.NextRunAt == nil {
				t.Fatalf("enable %v", err)
			}
			iv := int64(60)
			u, err := m.Update(ctx, s.ID, nil, UpdateRequest{IntervalSeconds: &iv})
			if err != nil || u.Kind() != KindInterval || u.CronExpr != "" {
				t.Fatalf("update %v %+v", err, u)
			}

			// Claim: force due.
			past := time.Now().Unix() - 5
			u.NextRunAt = &past
			if err := st.UpdateSchedule(ctx, u); err != nil {
				t.Fatal(err)
			}
			c1, err := st.ClaimDueSchedule(ctx, "w1", time.Minute)
			if err != nil || c1 == nil || strOr(c1.LockedBy) != "w1" {
				t.Fatalf("claim %v %+v", err, c1)
			}
			if c2, _ := st.ClaimDueSchedule(ctx, "w2", time.Minute); c2 != nil {
				t.Fatal("double claim")
			}
			// Release fenced by another worker is a no-op.
			_ = st.ReleaseSchedule(ctx, c1.ID, Release{WorkerID: "w2", NextRunAt: &past})
			if c3, _ := st.ClaimDueSchedule(ctx, "w2", time.Minute); c3 != nil {
				t.Fatal("fenced release leaked")
			}
			// Stale lock is reclaimable.
			if c4, _ := st.ClaimDueSchedule(ctx, "w2", -time.Second); c4 == nil {
				t.Fatal("stale reclaim failed")
			}

			r := &ScheduleRun{ID: NewScheduleRunID(), ScheduleID: s.ID, UserID: &owner, Attempt: 1, Status: RunStatusRunning, CreatedAt: time.Now().Unix()}
			if err := st.CreateRun(ctx, r); err != nil {
				t.Fatal(err)
			}
			r.Status, r.Output = RunStatusSuccess, map[string]any{"content": "ok"}
			if err := st.UpdateRun(ctx, r); err != nil {
				t.Fatal(err)
			}
			runs, total, err := m.GetRuns(ctx, s.ID, &owner, 10, 1)
			if err != nil || total != 1 || runs[0].Output["content"] != "ok" {
				t.Fatalf("runs %v %d", err, total)
			}
			if err := m.Delete(ctx, s.ID, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := st.GetRun(ctx, r.ID, nil); !errors.Is(err, ErrNotFound) {
				t.Fatal("runs not deleted")
			}
		})
	}
}

func TestSchedulerRetriesAndOneShot(t *testing.T) {
	for name, st := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var calls int32
			exec := NewComponentExecutor().RegisterAgent("a1", RunnerFunc(func(ctx context.Context, r RunRequest) (*RunOutput, error) {
				if atomic.AddInt32(&calls, 1) < 3 {
					return nil, errors.New("boom")
				}
				return &RunOutput{RunID: "r1", Content: "done:" + r.Input}, nil
			}))
			sch := New(st, exec)
			sch.Sleep = func(context.Context, time.Duration) error { return nil }
			m := NewManager(st)
			runAt := time.Now().Unix() - 1
			retries := 3
			s, err := m.Create(ctx, CreateRequest{Name: "once", RunAt: &runAt, TargetType: "agent", TargetID: "a1",
				MaxRetries: &retries, Payload: map[string]any{"message": "go"}})
			if err != nil {
				t.Fatal(err)
			}
			if n := sch.PollOnce(ctx); n != 1 {
				t.Fatalf("claimed %d", n)
			}
			sch.Wait()
			runs, total, _ := st.ListRuns(ctx, s.ID, nil, 10, 1)
			if total != 3 {
				t.Fatalf("want 3 attempts, got %d", total)
			}
			if runs[0].Status != RunStatusSuccess || runs[0].Output["content"] != "done:go" || strOr(runs[0].RunID) != "r1" {
				t.Fatalf("last run %+v", runs[0])
			}
			if runs[1].Status != RunStatusFailed {
				t.Fatalf("earlier run %+v", runs[1])
			}
			after, _ := st.GetSchedule(ctx, s.ID, nil)
			if after.Enabled || after.NextRunAt != nil || after.LockedBy != nil {
				t.Fatalf("one-shot should be disabled+released: %+v", after)
			}
		})
	}
}

func TestSchedulerIntervalCancelTimeout(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	started := make(chan struct{}, 1)
	exec := ExecutorFunc(func(ctx context.Context, s *Schedule) (*Result, error) {
		if s.Name == "slow" {
			started <- struct{}{}
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	sch := New(st, exec)
	m := NewManager(st)
	iv := int64(30)
	s, _ := m.Create(ctx, CreateRequest{Name: "slow", IntervalSeconds: iv, Endpoint: "/x"})
	past := time.Now().Unix() - 1
	s.NextRunAt = &past
	_ = st.UpdateSchedule(ctx, s)

	sch.PollOnce(ctx)
	<-started
	if n := sch.Cancel(s.ID); n != 1 {
		t.Fatalf("cancelled %d", n)
	}
	_ = sch.Stop(ctx)
	runs, _, _ := st.ListRuns(ctx, s.ID, nil, 10, 1)
	if len(runs) != 1 || runs[0].Status != RunStatusCancelled {
		t.Fatalf("runs %+v", runs)
	}
	after, _ := st.GetSchedule(ctx, s.ID, nil)
	if after.NextRunAt == nil || *after.NextRunAt <= time.Now().Unix() || !after.Enabled {
		t.Fatalf("interval not rescheduled: %+v", after)
	}

	one := 1
	s2, _ := m.Create(ctx, CreateRequest{Name: "to", Endpoint: "/y", TimeoutSeconds: &one})
	run, err := sch.Trigger(ctx, s2.ID, nil)
	if err != nil || run.Status != RunStatusTimeout {
		t.Fatalf("timeout run %v %+v", err, run)
	}
}

func TestHTTPExecutor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.URL.Path == "/agents/a1/runs" && r.Form.Get("stream") == "false" && r.Form.Get("message") == "hi" &&
			r.Header.Get("Authorization") == "Bearer tok" {
			_ = json.NewEncoder(w).Encode(map[string]any{"run_id": "r9", "session_id": "s9", "content": "hello", "status": "COMPLETED"})
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()
	ex := &HTTPExecutor{BaseURL: srv.URL, Token: "tok"}
	res, err := ex.Execute(context.Background(), &Schedule{Method: "POST", Endpoint: "/agents/a1/runs", Payload: map[string]any{"message": "hi"}})
	if err != nil || res.Status != RunStatusSuccess || res.RunID != "r9" || res.Output["content"] != "hello" {
		t.Fatalf("%v %+v", err, res)
	}
	res, _ = ex.Execute(context.Background(), &Schedule{Method: "GET", Endpoint: "/other"})
	if res.Status != RunStatusFailed || *res.StatusCode != 500 {
		t.Fatalf("%+v", res)
	}
}

func TestRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := NewMemoryStore()
	exec := NewComponentExecutor().RegisterAgent("a1", RunnerFunc(func(ctx context.Context, r RunRequest) (*RunOutput, error) {
		return &RunOutput{Content: "ok"}, nil
	}))
	r := gin.New()
	RegisterRoutes(r.Group("/"), NewManager(st), RouterOptions{Scheduler: New(st, exec)})

	do := func(method, path string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	code, created := do("POST", "/schedules", map[string]any{"name": "daily", "cron_expr": "0 8 * * *", "endpoint": "/agents/a1/runs", "payload": map[string]any{"message": "x"}})
	if code != 201 {
		t.Fatalf("create %d %v", code, created)
	}
	id := created["id"].(string)
	if code, _ := do("POST", "/schedules", map[string]any{"name": "bad", "cron_expr": "x", "endpoint": "/a"}); code != 422 {
		t.Fatalf("invalid cron %d", code)
	}
	if code, out := do("GET", "/schedules?enabled=true", nil); code != 200 || out["meta"].(map[string]any)["total_count"].(float64) != 1 {
		t.Fatalf("list %d %v", code, out)
	}
	if code, out := do("PATCH", "/schedules/"+id, map[string]any{"cron_expr": "0 9 * * *"}); code != 200 || out["cron_expr"] != "0 9 * * *" {
		t.Fatalf("patch %d %v", code, out)
	}
	if code, out := do("POST", "/schedules/"+id+"/trigger", nil); code != 200 || out["status"] != "success" {
		t.Fatalf("trigger %d %v", code, out)
	}
	code, runs := do("GET", "/schedules/"+id+"/runs", nil)
	if code != 200 || len(runs["data"].([]any)) != 1 {
		t.Fatalf("runs %d %v", code, runs)
	}
	runID := runs["data"].([]any)[0].(map[string]any)["id"].(string)
	if code, _ := do("GET", "/schedules/"+id+"/runs/"+runID, nil); code != 200 {
		t.Fatalf("get run %d", code)
	}
	if code, out := do("POST", "/schedules/"+id+"/disable", nil); code != 200 || out["enabled"] != false {
		t.Fatalf("disable %d", code)
	}
	if code, _ := do("POST", "/schedules/"+id+"/trigger", nil); code != 409 {
		t.Fatalf("trigger disabled %d", code)
	}
	if code, _ := do("DELETE", "/schedules/"+id, nil); code != 204 {
		t.Fatalf("delete %d", code)
	}
	if code, _ := do("GET", "/schedules/"+id, nil); code != 404 {
		t.Fatalf("get deleted %d", code)
	}
}
