package discovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"
)

type fakeProber struct {
	mu    sync.Mutex
	alive map[string]bool
	calls int
	fail  error
}

func newFakeProber(aliveHosts ...string) *fakeProber {
	p := &fakeProber{alive: make(map[string]bool)}
	for _, host := range aliveHosts {
		p.alive[host] = true
	}
	return p
}

func (f *fakeProber) Probe(parentCtx context.Context, host string, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail != nil {
		return f.fail
	}
	if parentCtx.Err() != nil {
		return parentCtx.Err()
	}
	if f.alive[host] {
		return nil
	}
	return errors.New("not alive")
}

func (f *fakeProber) setAlive(host string, alive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alive[host] = alive
}

func (f *fakeProber) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func waitForEvent(t *testing.T, events <-chan Event, timeout time.Duration) (Event, bool) {
	t.Helper()
	select {
	case evt, ok := <-events:
		return evt, ok
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event")
		return Event{}, false
	}
}

func TestScannerEmitsFoundOnFirstRound(t *testing.T) {
	prober := newFakeProber("192.168.1.10")
	hosts := []string{"192.168.1.10", "192.168.1.11"}
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return hosts, nil }),
		WithInterval(24*time.Hour),
		WithLogger(silentLogger()),
		WithClock(func() time.Time { return time.Unix(1700000000, 0) }),
	)
	events := scanner.Watch(t.Context())
	evt, _ := waitForEvent(t, events, time.Second)
	if evt.Type != EventFound {
		t.Fatalf("expected Found, got %s", evt.Type)
	}
	if evt.Console.IP != "192.168.1.10" || evt.Console.Port != DefaultPort {
		t.Fatalf("unexpected console %+v", evt.Console)
	}
	if evt.Console.SeenAt.IsZero() || evt.Console.LastPing.IsZero() {
		t.Fatalf("timestamps should be set")
	}
}

func TestScannerEmitsLostAfterThreshold(t *testing.T) {
	prober := newFakeProber("192.168.1.10")
	hosts := []string{"192.168.1.10"}
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return hosts, nil }),
		WithInterval(10*time.Millisecond),
		WithMissThreshold(2),
		WithLogger(silentLogger()),
	)
	events := scanner.Watch(t.Context())
	found, _ := waitForEvent(t, events, time.Second)
	if found.Type != EventFound {
		t.Fatalf("expected Found, got %s", found.Type)
	}
	prober.setAlive("192.168.1.10", false)
	lost, _ := waitForEvent(t, events, 2*time.Second)
	if lost.Type != EventLost {
		t.Fatalf("expected Lost, got %s", lost.Type)
	}
	if lost.Console.IP != "192.168.1.10" {
		t.Fatalf("lost event has wrong IP: %s", lost.Console.IP)
	}
}

func TestScannerDoesNotEmitLostBeforeThreshold(t *testing.T) {
	prober := newFakeProber("192.168.1.10")
	hosts := []string{"192.168.1.10"}
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return hosts, nil }),
		WithInterval(5*time.Millisecond),
		WithMissThreshold(1000),
		WithLogger(silentLogger()),
	)
	events := scanner.Watch(t.Context())
	_, _ = waitForEvent(t, events, time.Second)
	prober.setAlive("192.168.1.10", false)
	select {
	case evt := <-events:
		if evt.Type == EventLost {
			t.Fatalf("Lost emitted before threshold (calls=%d)", prober.callCount())
		}
	case <-time.After(120 * time.Millisecond):
	}
}

func TestScannerClosesChannelOnCancel(t *testing.T) {
	prober := newFakeProber()
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return []string{"10.0.0.1"}, nil }),
		WithInterval(10*time.Millisecond),
		WithLogger(silentLogger()),
	)
	parentCtx, cancel := context.WithCancel(t.Context())
	events := scanner.Watch(parentCtx)
	cancel()
	select {
	case _, ok := <-events:
		if ok {
			drain(events)
		}
	case <-time.After(time.Second):
		t.Fatalf("channel not closed after cancel")
	}
}

func TestScannerHandlesHostsError(t *testing.T) {
	prober := newFakeProber()
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return nil, errors.New("no interface") }),
		WithInterval(10*time.Millisecond),
		WithLogger(silentLogger()),
	)
	events := scanner.Watch(t.Context())
	select {
	case evt := <-events:
		t.Fatalf("unexpected event on hosts error: %+v", evt)
	case <-time.After(60 * time.Millisecond):
	}
}

func TestScannerFoundNotRepeated(t *testing.T) {
	prober := newFakeProber("192.168.1.10")
	hosts := []string{"192.168.1.10"}
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return hosts, nil }),
		WithInterval(10*time.Millisecond),
		WithLogger(silentLogger()),
	)
	events := scanner.Watch(t.Context())
	first, _ := waitForEvent(t, events, time.Second)
	if first.Type != EventFound {
		t.Fatalf("expected Found, got %s", first.Type)
	}
	select {
	case evt := <-events:
		t.Fatalf("unexpected repeat event: %+v", evt)
	case <-time.After(60 * time.Millisecond):
	}
}

func TestScannerMaxParallelBoundsConcurrency(t *testing.T) {
	var (
		mu      sync.Mutex
		inFly   int
		maxSeen int
	)
	prober := proberFunc(func(parentCtx context.Context, _ string, _ int) error {
		mu.Lock()
		inFly++
		if inFly > maxSeen {
			maxSeen = inFly
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		inFly--
		mu.Unlock()
		return errors.New("nope")
	})
	hosts := make([]string, 20)
	for i := range hosts {
		hosts[i] = "10.0.0." + strconv.Itoa(i+1)
	}
	scanner := NewScanner(
		prober,
		WithHosts(func() ([]string, error) { return hosts, nil }),
		WithInterval(24*time.Hour),
		WithMaxParallel(4),
		WithLogger(silentLogger()),
	)
	_ = scanner.Watch(t.Context())
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if maxSeen > 4 {
		t.Fatalf("expected max 4 concurrent probes, saw %d", maxSeen)
	}
	if maxSeen == 0 {
		t.Fatalf("prober was never called")
	}
}

type proberFunc func(parentCtx context.Context, host string, port int) error

func (f proberFunc) Probe(parentCtx context.Context, host string, port int) error {
	return f(parentCtx, host, port)
}

func drain(events <-chan Event) {
	for range events {
	}
}
