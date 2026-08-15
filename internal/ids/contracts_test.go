package ids

import "testing"

func TestMintHasPrefixAndFixedLength(t *testing.T) {
	id, err := Mint("obj")
	if err != nil || Validate("obj", id) != nil {
		t.Fatalf("invalid minted id %q: %v", id, err)
	}
}

func TestValidateRejectsWrongPrefix(t *testing.T) {
	if Validate("obj", "usr_00000000000000000000000000") == nil {
		t.Fatal("wrong prefix accepted")
	}
}

func TestValidateRejectsBadAlphabet(t *testing.T) {
	if Validate("obj", "obj_0000000000000000000000000!") == nil {
		t.Fatal("bad alphabet accepted")
	}
}

func TestMintIsUnpredictable(t *testing.T) {
	a, err := Mint("obj")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		b, err := Mint("obj")
		if err != nil {
			t.Fatal(err)
		}
		if a == b {
			t.Fatal("duplicate minted id")
		}
	}
}
