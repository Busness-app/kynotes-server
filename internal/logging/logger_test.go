package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDisallowedFieldsNeverReachOutput(t *testing.T) {
	var b bytes.Buffer
	New(&b, "info", "json").Info("x", "secret", "do-not-log", "event", "ok")
	if strings.Contains(b.String(), "do-not-log") || strings.Contains(b.String(), "secret") {
		t.Fatal(b.String())
	}
	var v map[string]any
	if json.Unmarshal(b.Bytes(), &v) != nil {
		t.Fatal("not json")
	}
}
