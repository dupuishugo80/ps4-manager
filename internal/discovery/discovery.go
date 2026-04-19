// Package discovery detects PS4s on the local network by probing RPI port 12800.
package discovery

import (
	"context"
	"errors"
	"time"
)

const DefaultPort = 12800

type EventType string

const (
	EventFound EventType = "found"
	EventLost  EventType = "lost"
)

type Console struct {
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	SeenAt   time.Time `json:"seen_at"`
	LastPing time.Time `json:"last_ping"`
}

func (c Console) Addr() string {
	return joinHostPort(c.IP, c.Port)
}

type Event struct {
	Type    EventType `json:"type"`
	Console Console   `json:"console"`
}

type Prober interface {
	Probe(parentCtx context.Context, host string, port int) error
}

var ErrNotRPI = errors.New("not a ps4 rpi service")
