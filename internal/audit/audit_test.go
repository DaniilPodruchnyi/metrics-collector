package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type testObserver struct {
	mu     sync.Mutex
	events []Event
}

func (o *testObserver) OnAudit(_ context.Context, e Event) {
	o.mu.Lock()
	o.events = append(o.events, e)
	o.mu.Unlock()
}

func TestSubject_SubscribeAndNotify(t *testing.T) {
	s := NewSubject()
	if s.HasObservers() {
		t.Fatalf("expected no observers initially")
	}

	var o testObserver
	s.Subscribe(&o)
	if !s.HasObservers() {
		t.Fatalf("expected observers after subscribe")
	}

	e := Event{TS: time.Now().Unix(), Metrics: []string{"Alloc"}, IPAddress: "127.0.0.1"}
	s.Notify(context.Background(), e)

	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(o.events))
	}
	if o.events[0].IPAddress != "127.0.0.1" {
		t.Fatalf("unexpected ip: %s", o.events[0].IPAddress)
	}
}

func TestSubject_SubscribeNil_NoPanic(t *testing.T) {
	s := NewSubject()
	s.Subscribe(nil)
	if s.HasObservers() {
		t.Fatalf("expected no observers after nil subscribe")
	}
}

func TestFileObserver_AppendsJSONLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	o := NewFileObserver(path)
	if o == nil {
		t.Fatalf("expected non-nil observer")
	}

	e1 := Event{TS: 1, Metrics: []string{"A", "B"}, IPAddress: "10.0.0.1"}
	e2 := Event{TS: 2, Metrics: []string{"C"}, IPAddress: "10.0.0.2"}

	o.OnAudit(context.Background(), e1)
	o.OnAudit(context.Background(), e2)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(b))
	}

	var got1, got2 Event
	if err := json.Unmarshal([]byte(lines[0]), &got1); err != nil {
		t.Fatalf("unmarshal line1 failed: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &got2); err != nil {
		t.Fatalf("unmarshal line2 failed: %v", err)
	}
	if got1.IPAddress != "10.0.0.1" || got2.IPAddress != "10.0.0.2" {
		t.Fatalf("unexpected events: %+v %+v", got1, got2)
	}
}

func TestNewFileObserver_EmptyReturnsNil(t *testing.T) {
	if o := NewFileObserver(""); o != nil {
		t.Fatalf("expected nil observer for empty path")
	}
}

func TestHTTPObserver_PostsJSON(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	o := NewHTTPObserver(ts.URL)
	if o == nil {
		t.Fatalf("expected non-nil observer")
	}

	e := Event{TS: 3, Metrics: []string{"X"}, IPAddress: "192.168.0.42"}
	o.OnAudit(context.Background(), e)

	mu.Lock()
	defer mu.Unlock()
	if len(gotBody) == 0 {
		t.Fatalf("expected request body to be set")
	}
	var decoded Event
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.IPAddress != "192.168.0.42" {
		t.Fatalf("unexpected decoded event: %+v", decoded)
	}
}

func TestNewHTTPObserver_EmptyReturnsNil(t *testing.T) {
	if o := NewHTTPObserver(""); o != nil {
		t.Fatalf("expected nil observer for empty url")
	}
}
