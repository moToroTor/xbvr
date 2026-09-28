package scrape

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gocolly/colly/v2"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsTransientVisitError(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		err  error
		want bool
	}{
		{"server error", 500, nil, true},
		{"bad gateway", 502, errors.New("Bad Gateway"), true},
		{"rate limited is separate", 429, errors.New("Too Many Requests"), false},
		{"blocked", 403, errors.New("Forbidden"), false},
		{"gone", 404, nil, false},
		{"ok", 200, nil, false},
		{"timeout interface", 0, timeoutErr{}, true},
		{"client timeout text", 0, errors.New(`Get "https://x/": Client.Timeout exceeded`), true},
		{"deadline text", 0, errors.New("context deadline exceeded"), true},
		{"reset text", 0, errors.New("read: connection reset by peer"), true},
		{"refused is permanent", 0, errors.New("connection refused"), false},
		{"unknown with status", 301, errors.New("redirect"), false},
		{"nil error", 0, nil, false},
	} {
		if got := isTransientVisitError(tc.code, tc.err); got != tc.want {
			t.Errorf("%s: isTransientVisitError(%d, %v) = %v, want %v",
				tc.name, tc.code, tc.err, got, tc.want)
		}
	}
}

// A page that fails transiently must be revisited and eventually ingested;
// a permanently failing page must be attempted exactly once (a 403 stays a
// single block signal, never a retry storm).
func TestTransientRetryRevisits(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		n := hits[r.URL.Path]
		mu.Unlock()
		switch r.URL.Path {
		case "/flaky":
			if n <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("recovered"))
		case "/dead":
			w.WriteHeader(http.StatusInternalServerError)
		case "/blocked":
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	c := colly.NewCollector()
	c = createCallbacks(c)

	var body string
	c.OnResponse(func(r *colly.Response) {
		if r.Request.URL.Path == "/flaky" {
			body = string(r.Body)
		}
	})

	// Visit returns the first attempt's error synchronously even when a
	// retry is queued, so outcomes are asserted on hit counts below.
	for _, p := range []string{"/flaky", "/dead", "/blocked"} {
		_ = c.Visit(srv.URL + p)
	}
	c.Wait()

	mu.Lock()
	defer mu.Unlock()
	if hits["/flaky"] != 3 {
		t.Errorf("/flaky visited %d times, want 3 (initial + 2 retries)", hits["/flaky"])
	}
	if body != "recovered" {
		t.Errorf("/flaky body = %q, want %q (retry never ingested the page)", body, "recovered")
	}
	if hits["/dead"] != 4 {
		t.Errorf("/dead visited %d times, want 4 (initial + 3 retries)", hits["/dead"])
	}
	if hits["/blocked"] != 1 {
		t.Errorf("/blocked visited %d times, want exactly 1 (blocks must not retry)", hits["/blocked"])
	}
}
