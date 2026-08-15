package ids

import (
	"crypto/rand"
	"errors"
	"strings"
)

const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

func Mint(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString(prefix)
	out.WriteByte('_')
	var n uint
	var bits uint
	for _, v := range b {
		n = (n << 8) | uint(v)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out.WriteByte(alphabet[(n>>bits)&31])
		}
	}
	if bits > 0 {
		out.WriteByte(alphabet[(n<<(5-bits))&31])
	}
	return out.String(), nil
}

func Validate(prefix, value string) error {
	if len(value) != len(prefix)+1+26 || !strings.HasPrefix(value, prefix+"_") {
		return errors.New("invalid id")
	}
	for _, c := range value[len(prefix)+1:] {
		if !strings.ContainsRune(alphabet, c) {
			return errors.New("invalid id")
		}
	}
	return nil
}
