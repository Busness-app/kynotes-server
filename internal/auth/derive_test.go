package auth

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestDeriveAuthSecretMatchesFixture(t *testing.T) {
	b, e := os.ReadFile("../../testdata/protocol/auth_vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var v []struct {
		Password, LoginSalt string
		Iterations          int
		AuthSecret          string
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	for _, x := range v {
		got, e := DeriveAuthSecret(x.Password, x.LoginSalt, x.Iterations)
		if e != nil || got != x.AuthSecret {
			t.Fatalf("fixture mismatch: %v %s", e, got)
		}
	}
}

func TestDeriveAuthSecretDeterministic(t *testing.T) {
	salt := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	a, e := DeriveAuthSecret("password", salt, MinLoginIterations)
	if e != nil {
		t.Fatal(e)
	}
	b, e := DeriveAuthSecret("password", salt, MinLoginIterations)
	if e != nil || a != b {
		t.Fatal("not deterministic")
	}
	if _, e = DeriveAuthSecret("password", "bad", MinLoginIterations); e == nil {
		t.Fatal("bad salt accepted")
	}
}
func TestHashVerify(t *testing.T) {
	h, e := HashAuthSecret("secret")
	if e != nil {
		t.Fatal(e)
	}
	if VerifyAuthSecret("secret", h) != nil {
		t.Fatal("valid secret rejected")
	}
	if VerifyAuthSecret("wrong", h) == nil {
		t.Fatal("wrong secret accepted")
	}
}
