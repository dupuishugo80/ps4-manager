package discovery

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultInterval       = 5 * time.Second
	defaultMaxParallel    = 32
	defaultMissesToLose   = 2
	defaultEventBufferCap = 16
)

type HostProvider func() ([]string, error)

type Scanner struct {
	prober        Prober
	hosts         HostProvider
	port          int
	interval      time.Duration
	maxParallel   int
	missThreshold int
	logger        *slog.Logger
	clock         func() time.Time
}

type Option func(*Scanner)

func WithHosts(provider HostProvider) Option {
	return func(s *Scanner) { s.hosts = provider }
}

func WithPort(port int) Option {
	return func(s *Scanner) { s.port = port }
}

func WithInterval(d time.Duration) Option {
	return func(s *Scanner) { s.interval = d }
}

func WithMaxParallel(n int) Option {
	return func(s *Scanner) { s.maxParallel = n }
}

func WithMissThreshold(n int) Option {
	return func(s *Scanner) { s.missThreshold = n }
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Scanner) { s.logger = logger }
}

func WithClock(clock func() time.Time) Option {
	return func(s *Scanner) { s.clock = clock }
}

func NewScanner(prober Prober, opts ...Option) *Scanner {
	scanner := &Scanner{
		prober:        prober,
		hosts:         func() ([]string, error) { return LocalHosts(nil) },
		port:          DefaultPort,
		interval:      defaultInterval,
		maxParallel:   defaultMaxParallel,
		missThreshold: defaultMissesToLose,
		logger:        slog.Default(),
		clock:         time.Now,
	}
	for _, opt := range opts {
		opt(scanner)
	}
	return scanner
}

// Watch runs scans in a loop and returns an event channel closed when parentCtx ends.
func (s *Scanner) Watch(parentCtx context.Context) <-chan Event {
	events := make(chan Event, defaultEventBufferCap)
	go s.loop(parentCtx, events)
	return events
}

func (s *Scanner) loop(parentCtx context.Context, out chan<- Event) {
	defer close(out)
	state := make(map[string]*Console)
	misses := make(map[string]int)
	s.runOnce(parentCtx, state, misses, out)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-parentCtx.Done():
			return
		case <-ticker.C:
			s.runOnce(parentCtx, state, misses, out)
		}
	}
}

func (s *Scanner) runOnce(parentCtx context.Context, state map[string]*Console, misses map[string]int, out chan<- Event) {
	hosts, err := s.hosts()
	if err != nil {
		s.logger.Warn("discovery hosts lookup failed", "error", err)
		return
	}
	if len(hosts) == 0 {
		return
	}
	alive := s.probeAll(parentCtx, hosts)
	now := s.clock()
	for ip := range alive {
		existing, known := state[ip]
		if known {
			existing.LastPing = now
			misses[ip] = 0
			continue
		}
		console := &Console{
			IP:       ip,
			Port:     s.port,
			SeenAt:   now,
			LastPing: now,
		}
		state[ip] = console
		misses[ip] = 0
		s.logger.Info("ps4 discovered", "ip", ip, "port", s.port)
		if !send(parentCtx, out, Event{Type: EventFound, Console: *console}) {
			return
		}
	}
	for ip, console := range state {
		if _, stillAlive := alive[ip]; stillAlive {
			continue
		}
		misses[ip]++
		missed := misses[ip]
		if missed < s.missThreshold {
			continue
		}
		lost := *console
		delete(state, ip)
		delete(misses, ip)
		s.logger.Info("ps4 lost", "ip", ip, "misses", missed)
		if !send(parentCtx, out, Event{Type: EventLost, Console: lost}) {
			return
		}
	}
}

func (s *Scanner) probeAll(parentCtx context.Context, hosts []string) map[string]struct{} {
	parallel := max(s.maxParallel, 1)
	sem := make(chan struct{}, parallel)
	var (
		mu    sync.Mutex
		alive = make(map[string]struct{})
		wg    sync.WaitGroup
	)
	for _, host := range hosts {
		if parentCtx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(h string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.prober.Probe(parentCtx, h, s.port); err != nil {
				s.logProbeError(h, err)
				return
			}
			mu.Lock()
			alive[h] = struct{}{}
			mu.Unlock()
		}(host)
	}
	wg.Wait()
	return alive
}

// Network errors are silenced: most /24 hosts are unreachable by design.
func (s *Scanner) logProbeError(host string, err error) {
	if errors.Is(err, ErrNotRPI) {
		s.logger.Debug("probe mismatch", "host", host)
	}
}

func send(parentCtx context.Context, out chan<- Event, evt Event) bool {
	select {
	case <-parentCtx.Done():
		return false
	case out <- evt:
		return true
	}
}
