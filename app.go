package main

import (
	"context"
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ps4-manager/internal/discovery"
)

type App struct {
	appCtx    context.Context
	cancelBg  context.CancelFunc
	discovery *discovery.Service
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(parentCtx context.Context) {
	a.appCtx = parentCtx
	bgCtx, cancel := context.WithCancel(parentCtx)
	a.cancelBg = cancel

	scanner := discovery.NewScanner(
		discovery.NewHTTPProber(),
		discovery.WithLogger(slog.Default()),
	)
	a.discovery = discovery.NewService(scanner, wailsEmitter{ctx: bgCtx})
	go a.discovery.Run(bgCtx)
}

func (a *App) shutdown(_ context.Context) {
	if a.cancelBg != nil {
		a.cancelBg()
	}
}

// GetConsoles returns the currently-alive PS4s detected on the network.
func (a *App) GetConsoles() []discovery.Console {
	if a.discovery == nil {
		return []discovery.Console{}
	}
	return a.discovery.Consoles()
}

type wailsEmitter struct {
	ctx context.Context
}

func (e wailsEmitter) Emit(name string, payload any) {
	runtime.EventsEmit(e.ctx, name, payload)
}
