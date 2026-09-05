package auth

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestHashAuthSecretIsArgon2idPHC(t *testing.T) {
	h, err := HashAuthSecret(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("want PHC argon2id, got %q", h)
	}
	if err := VerifyAuthSecret(strings.Repeat("a", 64), h); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := VerifyAuthSecret(strings.Repeat("b", 64), h); err == nil {
		t.Fatal("wrong secret verified")
	}
}

func TestVerifyAuthSecretRefusesScrypt(t *testing.T) {
	if err := VerifyAuthSecret("x", "scrypt$131072$8$1$AAAAAAAAAAAAAAAAAAAAAA==$AAAA"); err == nil {
		t.Fatal("legacy scrypt verifier must be refused, not verified")
	}
}

func TestVerifyAuthSecretReportsBusyDistinctly(t *testing.T) {
	h, err := HashAuthSecret(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- VerifyAuthSecret(strings.Repeat("a", 64), h)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrBusy) {
			t.Fatalf("got %v, want nil or ErrBusy", err)
		}
	}
}
