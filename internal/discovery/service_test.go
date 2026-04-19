package discovery

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeWatcher struct {
	events chan Event
}

func (f *fakeWatcher) Watch(parentCtx context.Context) <-chan Event {
	go func() {
		<-parentCtx.Done()
		close(f.events)
	}()
	return f.events
}

type fakeEmitter struct {
	mu       sync.Mutex
	captured []capturedEmit
}

type capturedEmit struct {
	name    string
	payload any
}

func (f *fakeEmitter) Emit(name string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.captured = append(f.captured, capturedEmit{name: name, payload: payload})
}

func (f *fakeEmitter) snapshot() []capturedEmit {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]capturedEmit, len(f.captured))
	copy(out, f.captured)
	return out
}

func TestServiceForwardsFoundAndTracksState(t *testing.T) {
	watcher := &fakeWatcher{events: make(chan Event, 4)}
	emitter := &fakeEmitter{}
	service := NewService(watcher, emitter)
	runCtx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		service.Run(runCtx)
		close(done)
	}()
	watcher.events <- Event{Type: EventFound, Console: Console{IP: "192.168.1.10", Port: DefaultPort}}
	watcher.events <- Event{Type: EventFound, Console: Console{IP: "192.168.1.11", Port: DefaultPort}}

	waitUntil(t, func() bool { return len(emitter.snapshot()) == 2 })

	consoles := service.Consoles()
	if len(consoles) != 2 || consoles[0].IP != "192.168.1.10" || consoles[1].IP != "192.168.1.11" {
		t.Fatalf("Consoles() = %+v", consoles)
	}
	for _, ev := range emitter.snapshot() {
		if ev.name != EventNameFound {
			t.Fatalf("unexpected event name %q", ev.name)
		}
	}
	cancel()
	<-done
}

func TestServiceForwardsLostAndRemovesFromState(t *testing.T) {
	watcher := &fakeWatcher{events: make(chan Event, 4)}
	emitter := &fakeEmitter{}
	service := NewService(watcher, emitter)
	runCtx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		service.Run(runCtx)
		close(done)
	}()
	watcher.events <- Event{Type: EventFound, Console: Console{IP: "10.0.0.5", Port: DefaultPort}}
	waitUntil(t, func() bool { return len(service.Consoles()) == 1 })
	watcher.events <- Event{Type: EventLost, Console: Console{IP: "10.0.0.5", Port: DefaultPort}}
	waitUntil(t, func() bool { return len(service.Consoles()) == 0 })

	events := emitter.snapshot()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].name != EventNameFound || events[1].name != EventNameLost {
		t.Fatalf("event names = %q, %q", events[0].name, events[1].name)
	}
	cancel()
	<-done
}

func TestServiceConsolesSnapshotIsIndependent(t *testing.T) {
	watcher := &fakeWatcher{events: make(chan Event, 4)}
	emitter := &fakeEmitter{}
	service := NewService(watcher, emitter)
	runCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go service.Run(runCtx)
	watcher.events <- Event{Type: EventFound, Console: Console{IP: "10.0.0.5"}}
	waitUntil(t, func() bool { return len(service.Consoles()) == 1 })

	snapshot := service.Consoles()
	snapshot[0].IP = "tampered"
	if service.Consoles()[0].IP != "10.0.0.5" {
		t.Fatalf("Consoles() mutated by external slice change")
	}
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition never satisfied within timeout")
}
