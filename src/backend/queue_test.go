package main

import (
	"testing"
	"time"
)

// TestQueueAbortDoesNotBlockWithoutARunningTask is a regression test for a
// deadlock where Abort held q.mutex while sending on abortedChannel, which is
// only received while a task is actively executing. A build between tasks
// (or one that hasn't started/has finished) has no listener, so the send -
// and every other Queue method waiting on q.mutex - would block forever.
func TestQueueAbortDoesNotBlockWithoutARunningTask(t *testing.T) {
	build := &Build{
		ID:             1,
		abortedChannel: make(chan string),
	}
	q := &Queue{running: []*Build{build}}

	done := make(chan error, 1)
	go func() {
		done <- q.Abort(1, StatusAborted)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Abort blocked instead of returning when no task was listening")
	}

	// The mutex must not still be held: every other Queue method must work.
	verifyDone := make(chan bool, 1)
	go func() { verifyDone <- q.Verify(1) }()
	select {
	case ok := <-verifyDone:
		if !ok {
			t.Error("expected build 1 to still be verifiable after Abort")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("q.mutex is still held after Abort returned")
	}
}

// TestQueueFlushLogsDoesNotBlockWithoutARunningTask mirrors the Abort
// regression test above for flushChannel.
func TestQueueFlushLogsDoesNotBlockWithoutARunningTask(t *testing.T) {
	build := &Build{
		ID:           2,
		flushChannel: make(chan bool),
	}
	q := &Queue{running: []*Build{build}}

	done := make(chan error, 1)
	go func() {
		done <- q.FlushLogs(2)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FlushLogs blocked instead of returning when no task was listening")
	}

	verifyDone := make(chan bool, 1)
	go func() { verifyDone <- q.Verify(2) }()
	select {
	case ok := <-verifyDone:
		if !ok {
			t.Error("expected build 2 to still be verifiable after FlushLogs")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("q.mutex is still held after FlushLogs returned")
	}
}

// TestQueueAbortDeliversToRunningTask verifies the fix doesn't break the
// normal case: a task actively receiving on abortedChannel must still get
// the abort signal.
func TestQueueAbortDeliversToRunningTask(t *testing.T) {
	build := &Build{
		ID:             3,
		abortedChannel: make(chan string),
	}
	q := &Queue{running: []*Build{build}}

	received := make(chan string, 1)
	go func() {
		received <- <-build.abortedChannel
	}()

	// Give the goroutine above a chance to start receiving.
	time.Sleep(50 * time.Millisecond)

	if err := q.Abort(3, StatusAborted); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case reason := <-received:
		if reason != StatusAborted {
			t.Errorf("expected reason %q, got %q", StatusAborted, reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("abort signal was not delivered to the running task")
	}
}

func TestQueueAbortReturnsErrorForUnknownBuild(t *testing.T) {
	q := &Queue{}
	if err := q.Abort(99, StatusAborted); err == nil {
		t.Error("expected an error for an unknown build id")
	}
}
