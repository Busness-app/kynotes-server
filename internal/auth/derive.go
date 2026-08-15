package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"strings"
)

const MinLoginIterations = 100000
const MaxLoginIterations = 12000000

func DeriveAuthSecret(password, salt string, iterations int) (string, error) {
	if err := validateLoginSalt(salt); err != nil {
		return "", err
	}
	if iterations < MinLoginIterations || iterations > MaxLoginIterations {
		return "", errors.New("invalid iterations")
	}
	raw, e := base64.StdEncoding.DecodeString(salt)
	if e != nil {
		return "", e
	}
	stretched := pbkdf2.Key([]byte(password), raw, iterations, 32, sha256.New)
	r := hkdf.New(sha256.New, stretched, nil, []byte("kynotes/auth/v1"))
	out := make([]byte, 32)
	if _, e = io.ReadFull(r, out); e != nil {
		return "", e
	}
	return hex.EncodeToString(out), nil
}
func validateLoginSalt(s string) error {
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil || len(b) < 16 || len(b) > 64 {
		return errors.New("invalid login salt")
	}
	return nil
}
func SyntheticLoginSalt(key, username string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("login-salt/v1\x00" + strings.ToLower(username)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)[:16])
}
