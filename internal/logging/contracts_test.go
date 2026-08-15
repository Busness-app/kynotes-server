package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestErrorStringIsNeverLogged(t *testing.T) {
	var b bytes.Buffer
	New(&b, "info", "json").Error("request failed", "error", "private-error-marker")
	if strings.Contains(b.String(), "private-error-marker") {
		t.Fatal("error attribute was logged")
	}
}

func TestOutputIsValidJSONLines(t *testing.T) {
	var b bytes.Buffer
	l := New(&b, "info", "json")
	l.Info("one", "event", "one")
	l.Info("two", "event", "two")
	for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
		var v map[string]any
		if json.Unmarshal([]byte(line), &v) != nil {
			t.Fatalf("invalid JSON line %q", line)
		}
	}
}
