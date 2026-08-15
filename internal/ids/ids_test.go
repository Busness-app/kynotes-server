package ids

import "testing"

func TestMintAndValidate(t *testing.T) {
	s, err := Mint("obj")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 30 {
		t.Fatalf("length %d", len(s))
	}
	if err = Validate("obj", s); err != nil {
		t.Fatal(err)
	}
	if Validate("usr", s) == nil {
		t.Fatal("wrong prefix accepted")
	}
}
