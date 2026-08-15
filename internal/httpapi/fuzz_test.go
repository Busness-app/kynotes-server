package httpapi

import (
	"strconv"
	"testing"
)

func FuzzParseCursor(f *testing.F) {
	f.Add("0")
	f.Fuzz(func(t *testing.T, s string) { _, _ = strconv.ParseInt(s, 10, 64) })
}
func FuzzParseChunkHeaders(f *testing.F) {
	f.Add("0")
	f.Fuzz(func(t *testing.T, s string) { _, _ = strconv.ParseInt(s, 10, 64) })
}
