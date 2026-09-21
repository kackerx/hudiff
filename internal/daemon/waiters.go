package daemon

import (
	"net/http"
	"sync"
	"time"

	"github.com/kackerx/hudiff/internal/session"
)

// rooms tracks who is long-polling each session, so the page can show
// "an agent is waiting" and submit knows whether anyone will pick it up.
type rooms struct {
	mu      sync.Mutex
	byPane  map[string]map[string]int
	changed chan struct{}
}

func newRooms() *rooms {
	return &rooms{byPane: map[string]map[string]int{}, changed: make(chan struct{})}
}

func (r *rooms) count(sid string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, v := range r.byPane[sid] {
		n += v
	}
	return n
}

func (r *rooms) enter(sid, pane string, delta int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byPane[sid] == nil {
		r.byPane[sid] = map[string]int{}
	}
	r.byPane[sid][pane] += delta
}

// Notify wakes every /api/wait; called after submit_seq changes.
func (r *rooms) Notify() {
	r.mu.Lock()
	defer r.mu.Unlock()
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *rooms) sub() chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.changed
}

func (r *rooms) wait(req *http.Request, sid, dir, pane string, since int, timeout time.Duration) int {
	r.enter(sid, pane, 1)
	defer r.enter(sid, pane, -1)
	deadline := time.After(timeout)
	for {
		ch := r.sub()
		if seq := session.ReadState(dir).SubmitSeq; seq > since {
			return seq
		}
		select {
		case <-ch:
		case <-deadline:
			return session.ReadState(dir).SubmitSeq
		case <-req.Context().Done():
			return session.ReadState(dir).SubmitSeq
		case <-time.After(5 * time.Second):
		}
	}
}
