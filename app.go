package main

import (
	"context"
	"fmt"
)

type App struct {
	appCtx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(parentCtx context.Context) {
	a.appCtx = parentCtx
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s", name)
}
