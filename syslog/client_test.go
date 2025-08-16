package syslog

import (
	"regexp"
	"strings"
	"testing"
)

func TestFormatRFC3164(t *testing.T) {
	r := &remoteSysClient{cfg: config{Facility: "daemon", Severity: "info", Tag: "mytag"}, appName: "myapp"}
	msg := "hello world\nnext"
	out := r.formatRFC3164(r.cfg.Facility, r.cfg.Severity, r.cfg.Tag, msg)

	// PRI for daemon/info == 3*8 + 6 = 30
	if !strings.HasPrefix(out, "<30>") {
		t.Fatalf("unexpected PRI prefix, got: %s", out)
	}

	// should contain the tag and escaped newline
	if !strings.Contains(out, "mytag:") {
		t.Fatalf("missing tag in rfc3164 output: %s", out)
	}
	if !strings.Contains(out, "hello world\\nnext") {
		t.Fatalf("message not escaped or present: %s", out)
	}
}

func TestFormatRFC5424(t *testing.T) {
	r := &remoteSysClient{cfg: config{Facility: "daemon", Severity: "info", Tag: "mytag"}, appName: "myapp"}
	msg := "test message"
	out := r.formatRFC5424(r.cfg.Facility, r.cfg.Severity, r.appName, msg)

	// PRI for daemon/info == 30 and version '1' present
	if !strings.HasPrefix(out, "<30>1 ") {
		t.Fatalf("unexpected RFC5424 prefix, got: %s", out)
	}

	parts := strings.Fields(out)
	if len(parts) < 7 {
		t.Fatalf("RFC5424 output has too few fields: %v", parts)
	}

	// parts[3] is APP-NAME (hostname is parts[2])
	if parts[3] != "myapp" {
		t.Fatalf("APP-NAME not present or wrong, got: %s", parts[3])
	}

	// msgid is parts[5] and should be hex (length 8)
	msgid := parts[5]
	matched, _ := regexp.MatchString("^[0-9a-fA-F]{8}$", msgid)
	if !matched {
		t.Fatalf("msgid not a hex string of length 8: %s", msgid)
	}

	// The message content should be at the end after a lone '-'
	if !strings.Contains(out, " - "+msg) {
		t.Fatalf("message not present at expected position: %s", out)
	}
}
