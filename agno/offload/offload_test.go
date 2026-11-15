package offload

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/fs"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func bigText(lines int) string {
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "line %d: value=%d\n", i, i*7)
	}
	b.WriteString("TOTAL: done")
	return b.String()
}

func TestOffloadRoundTrip(t *testing.T) {
	for name, store := range map[string]*ResultStore{
		"memory": NewMemoryResultStore(WithThresholdChars(100)),
		"local": func() *ResultStore {
			s, err := NewResultStore(fs.NewLocalBackend(t.TempDir()), WithThresholdChars(100))
			if err != nil {
				t.Fatal(err)
			}
			return s
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			out := bigText(1000)
			if !store.ShouldOffload("search", out) || store.ShouldOffload("read_result", out) {
				t.Fatal("threshold logic")
			}
			env := store.OffloadForModel(OffloadRequest{SessionID: "s1", RunID: "r1", ToolCallID: "c1", ToolName: "search", Output: out})
			if !strings.Contains(env, `<result id="res_`) || !strings.Contains(env, "TOTAL: done") || !strings.Contains(env, "lines omitted") {
				t.Fatalf("envelope: %s", env)
			}
			id := ResultIDFor("s1", "r1", "c1")
			payload, err := store.Payload(id)
			if err != nil || payload != out {
				t.Fatalf("payload mismatch: %v", err)
			}
			page, err := store.Read(id, 10, 12, 0, 0)
			if err != nil || page.Text != "line 10: value=70\nline 11: value=77\nline 12: value=84" || page.NextStartLine != 13 {
				t.Fatalf("page %+v %v", page, err)
			}
			matches, err := store.Search(id, `value=7\d\b`, 0)
			if err != nil || len(matches) == 0 || matches[0].LineNumber != 10 {
				t.Fatalf("search %+v %v", matches, err)
			}
			// Same call stored twice gets a fresh id instead of replacing the first.
			store.OffloadForModel(OffloadRequest{SessionID: "s1", RunID: "r1", ToolCallID: "c1", ToolName: "search", Output: out})
			refs, _ := store.LiveIDs("s1", 10)
			if len(refs) != 2 {
				t.Fatalf("live ids %d", len(refs))
			}
			n, err := store.DeleteForSessions("s1")
			if err != nil || n != 2 {
				t.Fatalf("delete %d %v", n, err)
			}
			if _, err := store.Payload(id); err == nil {
				t.Fatal("expected unknown id after delete")
			}
		})
	}
}

func TestPagingLongLine(t *testing.T) {
	line := strings.Repeat("a", 40000)
	p1 := PageFromContent(line, 1, 0, 0, 0)
	if len(p1.Text) != ReadMaxChars || p1.NextStartLine != 1 || p1.NextStartChar != ReadMaxChars {
		t.Fatalf("page1 %+v", p1.NextStartChar)
	}
	p3 := PageFromContent(line, 1, 0, 2*ReadMaxChars, 0)
	if len(p3.Text) != 40000-2*ReadMaxChars || p3.NextStartLine != 0 {
		t.Fatalf("page3 %d %d", len(p3.Text), p3.NextStartLine)
	}
}

func TestSearchClipAndCap(t *testing.T) {
	content := strings.Repeat("x", 2000) + "NEEDLE" + strings.Repeat("y", 2000)
	m := SearchContent(content, regexp.MustCompile("NEEDLE"), 0)
	if len(m) != 1 || m[0].CharOffset != 2000 || !strings.Contains(m[0].Line, "NEEDLE") || len([]rune(m[0].Line)) > SearchLineClip {
		t.Fatalf("clip %+v", m)
	}
	many := strings.Repeat("hit\n", 100)
	m = SearchContent(many, regexp.MustCompile("hit"), 0)
	if len(m) != SearchMaxMatches || !m[len(m)-1].More {
		t.Fatalf("cap %d", len(m))
	}
}

func TestTTLSweep(t *testing.T) {
	s := NewMemoryResultStore(WithThresholdChars(1), WithTTL(time.Second))
	s.OffloadForModel(OffloadRequest{SessionID: "s", RunID: "r", ToolCallID: "c", ToolName: "t", Output: "hello world"})
	n, _ := s.SweepExpired(time.Now().Add(2 * time.Second))
	if n != 1 {
		t.Fatalf("swept %d", n)
	}
}

type bigTool struct{ toolkit.Toolkit }

type bigParams struct {
	N int `json:"n"`
}

func (b *bigTool) Run(p bigParams) (interface{}, error) { return bigText(p.N), nil }

func TestWrapAndTools(t *testing.T) {
	store := NewMemoryResultStore(WithThresholdChars(200))
	bt := &bigTool{Toolkit: toolkit.NewToolkit()}
	bt.Name = "big"
	bt.Register("run", "produce output", bt, bt.Run, bigParams{})
	scope := StaticScope(Scope{SessionID: "sess", RunID: "run1", UserID: "u"})
	wrapped := Wrap(bt, store, scope)

	small, _ := wrapped.Execute("big_run", json.RawMessage(`{"n":2}`))
	if strings.Contains(small.(string), "<result") {
		t.Fatal("small output must stay inline")
	}
	out, _ := wrapped.Execute("big_run", json.RawMessage(`{"n":500}`))
	env := out.(string)
	re := regexp.MustCompile(`id="(res_[0-9a-f]+)"`)
	id := re.FindStringSubmatch(env)
	if id == nil {
		t.Fatalf("no envelope: %s", env)
	}
	tools := NewTools(store, scope)
	res, _ := tools.Execute("offload_read_result", json.RawMessage(fmt.Sprintf(`{"result_id":%q,"start_line":499,"end_line":499}`, id[1])))
	if !strings.Contains(res.(string), "line 499: value=3493") {
		t.Fatalf("read_result: %s", res)
	}
	res, _ = tools.Execute("offload_search_result", json.RawMessage(fmt.Sprintf(`{"result_id":%q,"pattern":"TOTAL"}`, id[1])))
	if !strings.Contains(res.(string), "1 match(es)") {
		t.Fatalf("search_result: %s", res)
	}
	other := NewTools(store, StaticScope(Scope{SessionID: "other"}))
	res, _ = other.Execute("offload_read_result", json.RawMessage(fmt.Sprintf(`{"result_id":%q}`, id[1])))
	if !strings.Contains(res.(string), "different session") {
		t.Fatalf("cross-session read must be refused: %s", res)
	}
}

func TestRefusedEnvelope(t *testing.T) {
	env := RenderRefusedEnvelope("t", bigText(100), "full", 5, 100)
	if !strings.Contains(env, `stored="false"`) || !strings.Contains(env, "NOT stored") || !strings.Contains(env, "TOTAL") {
		t.Fatalf("refused: %s", env)
	}
}
