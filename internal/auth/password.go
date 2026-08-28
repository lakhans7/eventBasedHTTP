package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost matches OWASP's current minimum recommendation for bcrypt.
// See docs/security.md for why bcrypt (not Argon2id) was chosen: it reuses
// the golang.org/x/crypto dependency already required elsewhere, and the
// hashing algorithm is fully abstracted behind this file so swapping to
// Argon2id later touches only HashPassword/VerifyPassword.
const bcryptCost = 12

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, plain string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
