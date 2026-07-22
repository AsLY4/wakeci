package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestServeUntilShutdownStopsServerAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	serveStarted := make(chan struct{})
	shutdownCalled := make(chan struct{})

	serve := func() error {
		close(serveStarted)
		<-shutdownCalled
		return http.ErrServerClosed
	}
	shutdown := func(context.Context) error {
		close(shutdownCalled)
		return nil
	}

	result := make(chan error, 1)
	go func() {
		result <- serveUntilShutdown(ctx, serve, shutdown)
	}()

	<-serveStarted
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("serveUntilShutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveUntilShutdown did not stop after cancellation")
	}
}

func TestServeUntilShutdownReturnsListenerErrors(t *testing.T) {
	want := errors.New("listen failed")
	err := serveUntilShutdown(context.Background(), func() error { return want }, func(context.Context) error {
		t.Fatal("shutdown called after listener failure")
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("serveUntilShutdown error = %v, want wrapped %v", err, want)
	}
}
