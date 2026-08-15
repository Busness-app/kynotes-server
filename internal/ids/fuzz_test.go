package ids

import "testing"

func FuzzParseID(f *testing.F) {
	f.Add("obj_test")
	f.Fuzz(func(t *testing.T, s string) { _ = Validate("obj", s) })
}
