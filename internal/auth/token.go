package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

const randomTokenLength = 32

type Token struct {
	Plaintext string
	Hash      []byte
}

func NewToken() (Token, error) {
	randomBytes := make([]byte, randomTokenLength)

	if _, err := rand.Read(randomBytes); err != nil {
		return Token{}, fmt.Errorf(
			"generate random token: %w",
			err,
		)
	}

	plaintext := base64.RawURLEncoding.EncodeToString(
		randomBytes,
	)

	return Token{
		Plaintext: plaintext,
		Hash:      HashToken(plaintext),
	}, nil
}

func HashToken(plaintext string) []byte {
	hash := sha256.Sum256([]byte(plaintext))

	result := make([]byte, len(hash))
	copy(result, hash[:])

	return result
}

func VerifyToken(plaintext string, expectedHash []byte) bool {
	if len(expectedHash) != sha256.Size {
		return false
	}

	actualHash := HashToken(plaintext)

	return subtle.ConstantTimeCompare(
		actualHash,
		expectedHash,
	) == 1
}
