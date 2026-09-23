package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyAway(t *testing.T) {
	h := readOnlyAway(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for _, c := range []struct {
		method, path, from string
		want               int
	}{
		{"GET", "/s/x/api/session", "172.16.0.9:5000", 200},
		{"GET", "/", "172.16.0.9:5000", 200},
		{"POST", "/s/x/api/deliver", "172.16.0.9:5000", 403},
		{"POST", "/s/x/api/comments", "172.16.0.9:5000", 403},
		{"DELETE", "/api/s/x", "172.16.0.9:5000", 403},
		{"GET", "/s/x/api/wait", "172.16.0.9:5000", 403},
		{"POST", "/s/x/api/deliver", "127.0.0.1:5000", 200},
		{"GET", "/s/x/api/wait", "[::1]:5000", 200},
	} {
		r := httptest.NewRequest(c.method, c.path, nil)
		r.RemoteAddr = c.from
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s %s from %s: %d, want %d", c.method, c.path, c.from, w.Code, c.want)
		}
	}
}

func TestAgentsHiddenFromAway(t *testing.T) {
	r := httptest.NewRequest("GET", "/s/x/api/agents", nil)
	r.RemoteAddr = "172.16.0.9:5000"
	w := httptest.NewRecorder()
	(&server{rooms: newRooms()}).apiAgents(w, r, "x", t.TempDir())
	if got := strings.TrimSpace(w.Body.String()); w.Code != 200 || got != `{"agents":[],"bound":null,"waiting":0}` {
		t.Errorf("remote agents: %d %s", w.Code, got)
	}
}
