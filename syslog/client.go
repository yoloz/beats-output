package syslog

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
)

// remoteSysClient sends RFC5424 messages to a remote syslog server over UDP/TCP,
// or falls back to local syslog when no network/host configured.
type remoteSysClient struct {
	cfg       config
	conn      net.Conn
	connMu    sync.Mutex
	localFall outputs.Client // optional local fallback client
	appName   string
}

func newSyslogClient(c config) (outputs.Client, error) {
	// If network and host are empty, keep using local syslog via previous implementation.
	if strings.TrimSpace(c.Network) == "" || strings.TrimSpace(c.Host) == "" {
		// Use local syslog writer implementation using stdlib
		// We'll keep it simple: create a small shim that writes RFC5424 to local syslog via UDP localhost:514
		// but to avoid side-effects, return a remoteSysClient with nil conn and no host -> will use local fallthrough.
		return &remoteSysClient{cfg: c, appName: c.Tag}, nil
	}

	// dial remote
	conn, err := net.DialTimeout(c.Network, c.Host, 5*time.Second)
	if err != nil {
		// if dial fails, still return client with no conn; Publish will try to reconnect
		return &remoteSysClient{cfg: c, conn: nil, appName: c.Tag}, nil
	}

	return &remoteSysClient{cfg: c, conn: conn, appName: c.Tag}, nil
}

func (r *remoteSysClient) Close() error {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
	if r.localFall != nil {
		_ = r.localFall.Close()
	}
	return nil
}

// formatRFC5424 builds a minimal RFC5424 message.
// PRI = facility*8 + severity
func (r *remoteSysClient) formatRFC5424(facility, severity string, appName string, msg string) string {
	pri := priFromFacilitySeverity(facility, severity)
	ts := time.Now().UTC().Format(time.RFC3339)
	// msgid: generate short random id
	id := make([]byte, 4)
	_, _ = rand.Read(id)
	msgid := hex.EncodeToString(id)
	// NILVALUE is '-'
	hostname := "-"
	procid := "-"
	if appName == "" {
		appName = "beats"
	}

	// Ensure message doesn't contain newlines (RFC5424 MSG can contain but for single-line it's simpler)
	safeMsg := strings.ReplaceAll(msg, "\n", "\\n")

	return fmt.Sprintf("<%d>1 %s %s %s %s %s - %s", pri, ts, hostname, appName, procid, msgid, safeMsg)
}

// formatRFC3164 builds a minimal RFC3164 message: "<PRI>TIMESTAMP HOST TAG: MSG"
// TIMESTAMP format: "Mmm dd hh:mm:ss" in local time
func (r *remoteSysClient) formatRFC3164(facility, severity string, tag string, msg string) string {
	pri := priFromFacilitySeverity(facility, severity)
	ts := time.Now().Format("Jan 2 15:04:05")
	hostname := "-"
	safeMsg := strings.ReplaceAll(msg, "\n", "\\n")
	if tag == "" {
		tag = "beats"
	}
	return fmt.Sprintf("<%d>%s %s %s: %s", pri, ts, hostname, tag, safeMsg)
}

func (r *remoteSysClient) connect() error {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	if r.conn != nil {
		return nil
	}
	if r.cfg.Network == "" || r.cfg.Host == "" {
		return nil
	}
	conn, err := net.DialTimeout(r.cfg.Network, r.cfg.Host, 5*time.Second)
	if err != nil {
		return err
	}
	r.conn = conn
	return nil
}

func (r *remoteSysClient) Publish(ctx context.Context, batch publisher.Batch) error {
	evs := batch.Events()

	// determine format: default to rfc3164 when not configured
	format := strings.ToLower(strings.TrimSpace(r.cfg.Format))
	if format == "" {
		format = "rfc3164"
	}

	// ensure connection
	if err := r.connect(); err != nil {
		// handle failure according to config
		r.handleFailure(batch)
		return err
	}

	r.connMu.Lock()
	conn := r.conn
	r.connMu.Unlock()

	// If no conn and no remote configured, fallback to local syslog using simple file/socket write
	if conn == nil {
		// Try to fallback to local syslog via UDP localhost:514
		remote := "udp"
		addr := "127.0.0.1:514"
		c := r.cfg
		conn, _ = net.DialTimeout(remote, addr, 2*time.Second)
		if conn == nil {
			// handle failure according to config
			r.handleFailure(batch)
			return fmt.Errorf("no remote connection and local syslog unreachable")
		}
		defer conn.Close()
		w := bufio.NewWriter(conn)
		for _, ev := range evs {
			msg := eventMessage(ev)
			var b string
			if format == "rfc5424" {
				b = r.formatRFC5424(c.Facility, c.Severity, r.appName, msg)
			} else {
				b = r.formatRFC3164(c.Facility, c.Severity, r.appName, msg)
			}
			_, err := w.WriteString(b + "\n")
			if err != nil {
				r.handleFailure(batch)
				return err
			}
		}
		_ = w.Flush()
		batch.ACK()
		return nil
	}

	// With an established conn, write messages according to the configured format.
	isTCP := r.cfg.Network == "tcp" || r.cfg.Network == "tcp4" || r.cfg.Network == "tcp6"
	writer := bufio.NewWriter(conn)
	for _, ev := range evs {
		msg := eventMessage(ev)
		var b string
		if format == "rfc5424" {
			b = r.formatRFC5424(r.cfg.Facility, r.cfg.Severity, r.appName, msg)
		} else {
			b = r.formatRFC3164(r.cfg.Facility, r.cfg.Severity, r.appName, msg)
		}
		var out string
		if isTCP && format == "rfc5424" {
			// octet counting: <len>MSG
			out = fmt.Sprintf("%d %s", len(b), b)
		} else {
			out = b
		}
		if _, err := writer.WriteString(out + "\n"); err != nil {
			// close conn and handle failure per config
			r.connMu.Lock()
			if r.conn != nil {
				_ = r.conn.Close()
				r.conn = nil
			}
			r.connMu.Unlock()
			r.handleFailure(batch)
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		r.handleFailure(batch)
		return err
	}

	batch.ACK()
	return nil
}

// handleFailure decides whether to Retry or ACK (discard) a batch based on config.
func (r *remoteSysClient) handleFailure(batch publisher.Batch) {
	mode := strings.ToLower(strings.TrimSpace(r.cfg.OnFailure))
	if mode == "retry" {
		batch.Retry()
		return
	}
	// default: discard (ACK)
	batch.ACK()
}

func eventMessage(ev publisher.Event) string {
	if ev.Content.Fields == nil {
		return ""
	}
	if m, err := ev.Content.Fields.GetValue("message"); err == nil {
		return fmt.Sprintf("%v", m)
	}
	return fmt.Sprintf("%v", ev.Content.Fields)
}

func (r *remoteSysClient) String() string {
	if r.cfg.Network != "" && r.cfg.Host != "" {
		return fmt.Sprintf("syslog(%s://%s)", r.cfg.Network, r.cfg.Host)
	}
	return "syslog(local)"
}

// priFromFacilitySeverity maps facility and severity to PRI numeric value.
func priFromFacilitySeverity(fac, sev string) int {
	facilityMap := map[string]int{
		"kern": 0, "user": 1, "mail": 2, "daemon": 3, "auth": 4,
		"syslog": 5, "lpr": 6, "news": 7, "uucp": 8, "cron": 9,
		"authpriv": 10, "ftp": 11, "ntp": 12,
		// Historical/alternate names
		"audit": 13, "security": 13, "console": 14,
		// local use facilities (RFC3164 local0..local7)
		"local0": 16, "local1": 17, "local2": 18, "local3": 19,
		"local4": 20, "local5": 21, "local6": 22, "local7": 23,
	}
	severityMap := map[string]int{
		"emerg": 0, "alert": 1, "crit": 2, "err": 3, "warning": 4,
		"notice": 5, "info": 6, "debug": 7,
	}
	fi := 3
	if v, ok := facilityMap[strings.ToLower(fac)]; ok {
		fi = v
	}
	si := 6
	if v, ok := severityMap[strings.ToLower(sev)]; ok {
		si = v
	}
	return fi*8 + si
}
