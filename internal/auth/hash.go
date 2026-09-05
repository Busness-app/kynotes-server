package auth

import (
	"errors"

	"github.com/Busness-app/ky-primitives/password"
)

var errInvalidSecret = errors.New("invalid secret")

// HashAuthSecret stores a login secret or recovery code as an Argon2id PHC string.
func HashAuthSecret(secret string) (string, error) { return password.Hash(secret) }

// VerifyAuthSecret is nil only when secret matches stored. A malformed or
// legacy (scrypt) verifier is an error, never a match.
func VerifyAuthSecret(secret, stored string) error {
	ok, err := password.Verify(secret, stored)
	if err != nil {
		return err
	}
	if !ok {
		return errInvalidSecret
	}
	return nil
}
