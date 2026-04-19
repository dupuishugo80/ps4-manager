// Debug CLI that runs the discovery scanner on the local network and prints events.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ps4-manager/internal/discovery"
)

func main() {
	interval := flag.Duration("interval", 3*time.Second, "delay between scans")
	timeout := flag.Duration("timeout", 500*time.Millisecond, "per-probe timeout")
	duration := flag.Duration("duration", 0, "total run duration (0 = until Ctrl+C)")
	parallel := flag.Int("parallel", 32, "max concurrent probes")
	misses := flag.Int("misses", 2, "consecutive misses before a console is declared lost")
	verbose := flag.Bool("v", false, "enable debug logs")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	parentCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		parentCtx, cancel = context.WithTimeout(parentCtx, *duration)
		defer cancel()
	}

	hosts, err := discovery.LocalHosts(nil)
	if err != nil {
		logger.Error("local hosts lookup failed", "error", err)
		os.Exit(1)
	}
	logger.Info("scanner starting", "hosts", len(hosts), "interval", interval.String(), "port", discovery.DefaultPort)

	scanner := discovery.NewScanner(
		discovery.NewHTTPProber(discovery.WithProbeTimeout(*timeout)),
		discovery.WithInterval(*interval),
		discovery.WithMaxParallel(*parallel),
		discovery.WithMissThreshold(*misses),
		discovery.WithLogger(logger),
	)

	for evt := range scanner.Watch(parentCtx) {
		fmt.Printf("%s  %-5s  %s (port %d)\n",
			evt.Console.LastPing.Format(time.RFC3339),
			evt.Type,
			evt.Console.IP,
			evt.Console.Port,
		)
	}
}
