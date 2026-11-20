package main

import (
	"context"
	"log/slog"

	"github.com/zellapay/zellapay/internal/wiring"
)

func runSimulator(c *wiring.Container, log *slog.Logger) (stop func()) {
	if c.Simulator == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx, log)
	}()
	return func() {
		cancel()
		<-done
	}
}
