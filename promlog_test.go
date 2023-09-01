package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// Capture output from go-kit/log.
// https://stackoverflow.com/a/26806093
func captureOutput(cb func()) string {
	var buf bytes.Buffer
	logger = log.NewLogfmtLogger(&buf)
	cb()
	return buf.String()
}

// Capture and verify basic promlog usage.
func TestPromlogBasic(t *testing.T) {

	output := captureOutput(func() {
		level.Info(logger).Log("msg", "Testdrive", "foo", 42.42)
	})
	if want, have := `level=info msg=Testdrive foo=42.42`, strings.TrimSpace(output); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}

}

// Capture and verify logging an error object.
func TestPromlogWithError(t *testing.T) {
	err := errors.New("test: Synthetic error")

	output := captureOutput(func() {
		level.Error(logger).Log("msg", "Something failed", "err", err)
	})
	if want, have := `level=error msg="Something failed" err="test: Synthetic error"`, strings.TrimSpace(output); want != have {
		t.Errorf("\nwant '%s'\nhave '%s'", want, have)
	}
}
