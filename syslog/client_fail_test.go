package syslog

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/elastic/beats/v7/libbeat/publisher"
)

// errConn always returns an error on Write to simulate a failing network.
type errConn struct{}

func (e *errConn) Read(b []byte) (int, error)         { return 0, io.EOF }
func (e *errConn) Write(b []byte) (int, error)        { return 0, errors.New("write error") }
func (e *errConn) Close() error                       { return nil }
func (e *errConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (e *errConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (e *errConn) SetDeadline(t time.Time) error      { return nil }
func (e *errConn) SetReadDeadline(t time.Time) error  { return nil }
func (e *errConn) SetWriteDeadline(t time.Time) error { return nil }

type mockBatch struct {
	evs     []publisher.Event
	Acked   bool
	Retried bool
}

func (m *mockBatch) Events() []publisher.Event                { return m.evs }
func (m *mockBatch) ACK()                                     { m.Acked = true }
func (m *mockBatch) Drop()                                    { m.Acked = true }
func (m *mockBatch) Retry()                                   { m.Retried = true }
func (m *mockBatch) RetryEvents(events []publisher.Event)     { m.Retried = true }
func (m *mockBatch) Cancelled()                               {}
func (m *mockBatch) CancelledEvents(events []publisher.Event) {}

func TestOnFailureDiscard(t *testing.T) {
	cfg := defaultConfig()
	cfg.OnFailure = "discard"
	c := &remoteSysClient{cfg: cfg, conn: &errConn{}, appName: "beats"}

	mb := &mockBatch{evs: []publisher.Event{{}}}
	err := c.Publish(context.Background(), mb)
	if err == nil {
		t.Fatalf("expected publish to return error when write fails")
	}
	if !mb.Acked {
		t.Fatalf("expected batch to be ACKed (discard) on failure, got Acked=%v Retried=%v", mb.Acked, mb.Retried)
	}
	if mb.Retried {
		t.Fatalf("did not expect Retry to be called for discard mode")
	}
}

func TestOnFailureRetry(t *testing.T) {
	cfg := defaultConfig()
	cfg.OnFailure = "retry"
	c := &remoteSysClient{cfg: cfg, conn: &errConn{}, appName: "beats"}

	mb := &mockBatch{evs: []publisher.Event{{}}}
	err := c.Publish(context.Background(), mb)
	if err == nil {
		t.Fatalf("expected publish to return error when write fails")
	}
	if !mb.Retried {
		t.Fatalf("expected batch to be retried on failure, got Acked=%v Retried=%v", mb.Acked, mb.Retried)
	}
	if mb.Acked {
		t.Fatalf("did not expect ACK to be called for retry mode")
	}
}
