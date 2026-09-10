package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	defaultArgonMemory      uint32 = 19 * 1024
	defaultArgonIterations  uint32 = 2
	defaultArgonParallelism uint8  = 1
	defaultSaltLength              = 16
	defaultKeyLength        uint32 = 32
)

type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  int
	keyLength   uint32
}

func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{
		memory:      defaultArgonMemory,
		iterations:  defaultArgonIterations,
		parallelism: defaultArgonParallelism,
		saltLength:  defaultSaltLength,
		keyLength:   defaultKeyLength,
	}
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	salt := make([]byte, h.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, h.iterations, h.memory, h.parallelism, h.keyLength)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.memory,
		h.iterations,
		h.parallelism,
		encodedSalt,
		encodedHash,
	), nil
}

func (h *PasswordHasher) Verify(password string, encodedHash string) (bool, error) {
	memory, iterations, parallelism, salt, expectedHash, err := parseArgon2Hash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}

func parseArgon2Hash(encodedHash string) (uint32, uint32, uint8, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return 0, 0, 0, nil, nil, errors.New(
			"invalid argon2id hash format",
		)
	}

	if parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, errors.New(
			"password hash is not argon2id",
		)
	}

	var version int
	if _, err := fmt.Sscanf(
		parts[2],
		"v=%d",
		&version,
	); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf(
			"parse argon2id version: %w",
			err,
		)
	}

	if version != argon2.Version {
		return 0, 0, 0, nil, nil, fmt.Errorf(
			"unsupported argon2id version: %d",
			version,
		)
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf(
			"parse argon2id parameters: %w",
			err,
		)
	}

	if memory < 8*1024 || memory > 256*1024 {
		return 0, 0, 0, nil, nil, errors.New(
			"argon2id memory parameter is outside allowed range",
		)
	}

	if iterations < 1 || iterations > 10 {
		return 0, 0, 0, nil, nil, errors.New(
			"argon2id iterations parameter is outside allowed range",
		)
	}

	if parallelism < 1 || parallelism > 16 {
		return 0, 0, 0, nil, nil, errors.New(
			"argon2id parallelism parameter is outside allowed range",
		)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf(
			"decode argon2id salt: %w",
			err,
		)
	}

	if len(salt) < 16 {
		return 0, 0, 0, nil, nil, errors.New(
			"argon2id salt is too short",
		)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf(
			"decode argon2id hash: %w",
			err,
		)
	}

	if len(expectedHash) < 16 || len(expectedHash) > 64 {
		return 0, 0, 0, nil, nil, errors.New(
			"argon2id hash length is outside allowed range",
		)
	}

	return memory, iterations, parallelism, salt, expectedHash, nil
}
