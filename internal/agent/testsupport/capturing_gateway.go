package testsupport

import (
	"context"
	"sync"
)

type Reply struct {
	To   string
	Text string
}

type CapturingGateway struct {
	mu      sync.Mutex
	replies []Reply
}

func (g *CapturingGateway) SendTextMessage(_ context.Context, toE164, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = append(g.replies, Reply{To: toE164, Text: text})
	return nil
}

func (g *CapturingGateway) LastReply() (Reply, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.replies) == 0 {
		return Reply{}, false
	}
	return g.replies[len(g.replies)-1], true
}

func (g *CapturingGateway) All() []Reply {
	g.mu.Lock()
	defer g.mu.Unlock()
	cp := make([]Reply, len(g.replies))
	copy(cp, g.replies)
	return cp
}

func (g *CapturingGateway) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = nil
}
