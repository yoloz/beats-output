package syslog

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/publisher"
)

// testBatch is a minimal publisher.Batch implementation for tests.
type testBatch struct {
	events  []publisher.Event
	acked   bool
	retried bool
}

func (b *testBatch) Events() []publisher.Event                { return b.events }
func (b *testBatch) ACK()                                     { b.acked = true }
func (b *testBatch) Drop()                                    {}
func (b *testBatch) Retry()                                   { b.retried = true }
func (b *testBatch) RetryEvents(events []publisher.Event)     { b.retried = true }
func (b *testBatch) Cancelled()                               {}
func (b *testBatch) CancelledEvents(events []publisher.Event) {}

func TestTCPOctetCountingRFC5424(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen tcp: %v", err)
	}
	defer ln.Close()

	acceptCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptCh <- "ERR:" + err.Error()
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		// read one line
		line, _ := r.ReadString('\n')
		acceptCh <- line
	}()

	addr := ln.Addr().String()
	cfg := config{Network: "tcp", Host: addr, Facility: "daemon", Severity: "info", Tag: "test", Format: "rfc5424"}
	clientIfc, err := newSyslogClient(cfg)
	if err != nil {
		t.Fatalf("newSyslogClient failed: %v", err)
	}
	client := clientIfc.(*remoteSysClient)
	defer client.Close()

	b := &testBatch{events: []publisher.Event{{Content: beat.Event{Fields: common.MapStr{"message": "hello-octet"}}}}}
	ctx := context.Background()
	if err := client.Publish(ctx, b); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	select {
	case line := <-acceptCh:
		if strings.HasPrefix(line, "ERR:") {
			t.Fatalf("server accept error: %s", line)
		}
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("unexpected framed tcp line: %q", line)
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			t.Fatalf("failed to parse length prefix: %v", err)
		}
		msg := parts[1]
		if n != len(msg) {
			t.Fatalf("octet count mismatch: prefix=%d len(msg)=%d msg=%q", n, len(msg), msg)
		}
		// ensure message looks like RFC5424 (starts with <PRI>1 )
		if !strings.HasPrefix(msg, "<30>1 ") {
			t.Fatalf("message not RFC5424 formatted: %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for tcp server to receive message")
	}
}

func TestUDPWriteRFC3164(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen udp: %v", err)
	}
	defer pc.Close()

	readCh := make(chan string, 1)
	go func() {
		buf := make([]byte, 65535)
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			readCh <- "ERR:" + err.Error()
			return
		}
		readCh <- string(buf[:n])
	}()

	addr := pc.LocalAddr().String()
	cfg := config{Network: "udp", Host: addr, Facility: "daemon", Severity: "info", Tag: "test", Format: "rfc3164"}
	clientIfc, err := newSyslogClient(cfg)
	if err != nil {
		t.Fatalf("newSyslogClient failed: %v", err)
	}
	client := clientIfc.(*remoteSysClient)
	defer client.Close()

	b := &testBatch{events: []publisher.Event{{Content: beat.Event{Fields: common.MapStr{"message": "hello-udp"}}}}}
	ctx := context.Background()
	if err := client.Publish(ctx, b); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	select {
	case pkt := <-readCh:
		if strings.HasPrefix(pkt, "ERR:") {
			t.Fatalf("udp read error: %s", pkt)
		}
		pkt = strings.TrimSpace(pkt)
		if !strings.HasPrefix(pkt, "<30>") {
			t.Fatalf("udp packet not starting with PRI <30>: %q", pkt)
		}
		// ensure tag present
		if !strings.Contains(pkt, "test:") {
			t.Fatalf("udp packet missing tag: %q", pkt)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for udp packet")
	}
}
