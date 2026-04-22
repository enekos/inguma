package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsage_noArgs(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{}, &out, &out)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Errorf("usage missing: %q", out.String())
	}
}

func TestUsage_unknown(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"bogus"}, &out, &out)
	if code != 2 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Errorf("unknown not mentioned: %q", out.String())
	}
}
