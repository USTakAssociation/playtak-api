package auth

import (
	"crypto/rand"

	"golang.org/x/crypto/bcrypt"
)

// bcrypt only uses the first 72 bytes of a password. Longer input is
// truncated rather than rejected, as jBCrypt (used by the Java server) did,
// so existing long passwords keep working.
const bcryptMaxBytes = 72

func hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword(bcryptInput(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// checkPassword reports whether password matches hash. The hash records its
// own cost, so hashes from the Java server verify unchanged.
func checkPassword(hash, password string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), bcryptInput(password)) == nil
}

func bcryptInput(password string) []byte {
	b := []byte(password)
	if len(b) > bcryptMaxBytes {
		return b[:bcryptMaxBytes]
	}
	return b
}

// newTemporaryPassword returns a random password emailed to new players.
func newTemporaryPassword() string {
	return rand.Text()
}
