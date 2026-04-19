package discovery

import (
	"context"
	"slices"
	"sync"
)

const (
	EventNameFound = "discovery:found"
	EventNameLost  = "discovery:lost"
)

type EventEmitter interface {
	Emit(name string, payload any)
}

type Watcher interface {
	Watch(parentCtx context.Context) <-chan Event
}

// Service bridges a scanner to a UI event bus while keeping an authoritative
// list of currently-alive consoles that the UI can query on demand.
type Service struct {
	watcher Watcher
	emitter EventEmitter

	mu    sync.RWMutex
	alive map[string]Console
}

func NewService(watcher Watcher, emitter EventEmitter) *Service {
	return &Service{
		watcher: watcher,
		emitter: emitter,
		alive:   make(map[string]Console),
	}
}

// Run consumes scanner events until parentCtx is cancelled.
func (s *Service) Run(parentCtx context.Context) {
	for evt := range s.watcher.Watch(parentCtx) {
		s.apply(evt)
		s.emitter.Emit(eventName(evt.Type), evt.Console)
	}
}

func (s *Service) apply(evt Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch evt.Type {
	case EventFound:
		s.alive[evt.Console.IP] = evt.Console
	case EventLost:
		delete(s.alive, evt.Console.IP)
	}
}

// Consoles returns a snapshot of the currently-alive consoles sorted by IP.
func (s *Service) Consoles() []Console {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Console, 0, len(s.alive))
	for _, c := range s.alive {
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Console) int {
		if a.IP < b.IP {
			return -1
		}
		if a.IP > b.IP {
			return 1
		}
		return 0
	})
	return out
}

func eventName(t EventType) string {
	if t == EventLost {
		return EventNameLost
	}
	return EventNameFound
}
