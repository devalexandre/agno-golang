package valkey

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

// fakeServer is a minimal RESP2 server recording commands; handler returns a
// raw RESP reply (or "" for +OK).
type fakeServer struct {
	mu       sync.Mutex
	cmds     [][]string
	handler  func(cmd []string) string
	listener net.Listener
}

func newFake(t *testing.T, h func([]string) string) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	f := &fakeServer{handler: h, listener: ln}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return f
}

func readCmd(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
	cmd := make([]string, n)
	for i := 0; i < n; i++ {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, _ := strconv.Atoi(strings.TrimSpace(l[1:]))
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		cmd[i] = string(buf[:size])
	}
	return cmd, nil
}

func (f *fakeServer) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		cmd, err := readCmd(r)
		if err != nil {
			return
		}
		name := strings.ToUpper(cmd[0])
		reply := "+OK\r\n"
		switch name {
		case "HELLO":
			reply = "-ERR unknown command 'HELLO'\r\n"
		case "CLIENT", "PING":
		default:
			f.mu.Lock()
			f.cmds = append(f.cmds, cmd)
			f.mu.Unlock()
			if f.handler != nil {
				if r := f.handler(cmd); r != "" {
					reply = r
				}
			}
		}
		if _, err := c.Write([]byte(reply)); err != nil {
			return
		}
	}
}

func (f *fakeServer) find(name string) [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out [][]string
	for _, c := range f.cmds {
		if strings.EqualFold(c[0], name) {
			out = append(out, c)
		}
	}
	return out
}

func bulk(s string) string { return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s) }

func newDB(t *testing.T, f *fakeServer, dist string) *Valkey {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: f.listener.Addr().String(), Protocol: 2, DisableIdentity: true})
	cfg := Config{IndexName: "docs", Client: client, Embedder: testembed.New(8)}
	if dist != "" {
		cfg.Distance = vectordb.Distance(dist)
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateSchema(t *testing.T) {
	f := newFake(t, func(cmd []string) string {
		if strings.EqualFold(cmd[0], "FT.INFO") {
			return "-Index with name 'docs' not found\r\n"
		}
		return ""
	})
	db := newDB(t, f, "")
	if err := db.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	creates := f.find("FT.CREATE")
	if len(creates) != 1 {
		t.Fatalf("expected one FT.CREATE, got %v", f.cmds)
	}
	got := strings.Join(creates[0], " ")
	for _, want := range []string{"docs ON HASH PREFIX 1 docs:", "name TAG", "content TEXT", "linked_to TAG",
		"embedding VECTOR HNSW 6 TYPE FLOAT32 DIM 8 DISTANCE_METRIC COSINE"} {
		if !strings.Contains(got, want) {
			t.Errorf("FT.CREATE missing %q: %s", want, got)
		}
	}
}

func TestInsertAndVectorSearch(t *testing.T) {
	f := newFake(t, func(cmd []string) string {
		if strings.EqualFold(cmd[0], "FT.SEARCH") {
			return "*5\r\n:2\r\n" + bulk("docs:a") +
				"*6\r\n" + bulk("id") + bulk("a") + bulk("content") + bulk("alpha") + bulk("score") + bulk("0.1") +
				bulk("docs:b") +
				"*6\r\n" + bulk("id") + bulk("b") + bulk("content") + bulk("beta") + bulk("score") + bulk("0.4")
		}
		if strings.EqualFold(cmd[0], "HSET") {
			return ":1\r\n"
		}
		return ""
	})
	db := newDB(t, f, "")
	ctx := context.Background()
	docs := []*document.Document{{ID: "a", Name: "n1", Content: "alpha", Metadata: map[string]interface{}{"category": "x", "id": "evil"}}}
	if err := db.Insert(ctx, docs, map[string]interface{}{"linked_to": "kb1"}); err != nil {
		t.Fatal(err)
	}
	hs := f.find("HSET")
	if len(hs) != 1 || hs[0][1] != "docs:a" {
		t.Fatalf("unexpected HSET %v", hs)
	}
	fields := map[string]string{}
	for i := 2; i+1 < len(hs[0]); i += 2 {
		fields[hs[0][i]] = hs[0][i+1]
	}
	if fields["id"] != "a" || fields["category"] != "x" || fields["linked_to"] != "kb1" || fields["name"] != "n1" {
		t.Errorf("unexpected hash fields: %v", fields)
	}
	if len(fields["embedding"]) != 32 {
		t.Errorf("embedding should be 8 float32 = 32 bytes, got %d", len(fields["embedding"]))
	}

	res, err := db.VectorSearch(ctx, "alpha", 2, map[string]interface{}{"category": []string{"x", "y z"}, "unknown": "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	q := f.find("FT.SEARCH")[0]
	if q[2] != `(@category:{x|y\ z})=>[KNN 2 @embedding $query_vector AS score]` {
		t.Errorf("unexpected query %q", q[2])
	}
	if len(res) != 2 || res[0].Document.ID != "a" || res[0].Score < 0.89 || res[0].Score > 0.91 {
		t.Fatalf("unexpected results %+v", res)
	}
}

func TestKeywordAndExists(t *testing.T) {
	f := newFake(t, func(cmd []string) string {
		switch strings.ToUpper(cmd[0]) {
		case "FT.SEARCH":
			if strings.HasPrefix(cmd[2], "@name:") {
				return "*1\r\n:3\r\n"
			}
			return "*3\r\n:1\r\n" + bulk("docs:k") + "*2\r\n" + bulk("content") + bulk("hello world")
		case "EXISTS":
			return ":1\r\n"
		}
		return ""
	})
	db := newDB(t, f, "l2")
	ctx := context.Background()
	res, err := db.KeywordSearch(ctx, "hello, wor-ld!", 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.find("FT.SEARCH")[0][2]; got != "@content:(hello world)" {
		t.Errorf("unexpected keyword query %q", got)
	}
	if len(res) != 1 || res[0].Document.ID != "k" {
		t.Errorf("unexpected keyword results %+v", res)
	}
	ok, err := db.NameExists(ctx, "my doc")
	if err != nil || !ok {
		t.Fatalf("NameExists = %v, %v", ok, err)
	}
	if got := f.find("FT.SEARCH")[1][2]; got != `@name:{my\ doc}` {
		t.Errorf("unexpected name query %q", got)
	}
	if ok, err := db.IDExists(ctx, "k"); err != nil || !ok {
		t.Fatalf("IDExists = %v, %v", ok, err)
	}
}

func TestDimensionMismatch(t *testing.T) {
	f := newFake(t, nil)
	db := newDB(t, f, "")
	err := db.Insert(context.Background(), []*document.Document{{ID: "x", Content: "c", Embeddings: []float64{1, 2}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("expected dimension error, got %v", err)
	}
}
