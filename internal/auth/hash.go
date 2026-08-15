package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/scrypt"
	"strconv"
	"strings"
)

const scryptN = 1 << 17
const scryptR = 8
const scryptP = 1
const scryptKeyLen = 32

var scryptSlots = make(chan struct{}, 4)

func takeScryptSlot() func() {
	scryptSlots <- struct{}{}
	return func() { <-scryptSlots }
}

func HashAuthSecret(secret string) (string, error) {
	release := takeScryptSlot()
	defer release()
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	sum, e := scrypt.Key([]byte(secret), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("scrypt$%d$%d$%d$%s$%s", scryptN, scryptR, scryptP, base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(sum)), nil
}
func VerifyAuthSecret(secret, stored string) error {
	p := strings.Split(stored, "$")
	if len(p) != 6 || p[0] != "scrypt" {
		return errors.New("invalid verifier")
	}
	n, e := strconv.Atoi(p[1])
	if e != nil {
		return e
	}
	r, e := strconv.Atoi(p[2])
	if e != nil {
		return e
	}
	cost, e := strconv.Atoi(p[3])
	if e != nil {
		return e
	}
	salt, e := base64.StdEncoding.DecodeString(p[4])
	if e != nil {
		return e
	}
	want, e := base64.StdEncoding.DecodeString(p[5])
	if e != nil {
		return e
	}
	if n < 2 || r < 1 || cost < 1 || len(salt) < 16 || len(want) == 0 || n > 1<<20 || r > 32 || cost > 8 {
		return errors.New("invalid verifier parameters")
	}
	release := takeScryptSlot()
	defer release()
	got, e := scrypt.Key([]byte(secret), salt, n, r, cost, len(want))
	if e != nil {
		return e
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return errors.New("invalid secret")
	}
	return nil
}
