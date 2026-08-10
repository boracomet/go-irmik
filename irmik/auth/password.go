package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// HashAlgo selects the password hashing algorithm.
type HashAlgo string

const (
	AlgoBcrypt  HashAlgo = "bcrypt"
	AlgoArgon2id HashAlgo = "argon2id"
)

// PasswordOptions configures hashing.
type PasswordOptions struct {
	Algo         HashAlgo
	BcryptCost   int
	ArgonTime    uint32
	ArgonMemory  uint32 // KiB
	ArgonThreads uint8
	ArgonKeyLen  uint32
}

func (o PasswordOptions) withDefaults() PasswordOptions {
	if o.Algo == "" {
		o.Algo = AlgoArgon2id
	}
	if o.BcryptCost == 0 {
		o.BcryptCost = bcrypt.DefaultCost
	}
	if o.ArgonTime == 0 {
		o.ArgonTime = 1
	}
	if o.ArgonMemory == 0 {
		o.ArgonMemory = 64 * 1024
	}
	if o.ArgonThreads == 0 {
		o.ArgonThreads = 4
	}
	if o.ArgonKeyLen == 0 {
		o.ArgonKeyLen = 32
	}
	return o
}

// HashPassword hashes password with the configured algorithm.
// Encoded form: "bcrypt$<hash>" or "argon2id$<params>$<salt>$<hash>".
func HashPassword(password string, opts PasswordOptions) (string, error) {
	opts = opts.withDefaults()
	switch opts.Algo {
	case AlgoBcrypt:
		b, err := bcrypt.GenerateFromPassword([]byte(password), opts.BcryptCost)
		if err != nil {
			return "", err
		}
		return "bcrypt$" + string(b), nil
	case AlgoArgon2id:
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return "", err
		}
		hash := argon2.IDKey([]byte(password), salt, opts.ArgonTime, opts.ArgonMemory, opts.ArgonThreads, opts.ArgonKeyLen)
		return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
			opts.ArgonMemory, opts.ArgonTime, opts.ArgonThreads,
			base64.RawStdEncoding.EncodeToString(salt),
			base64.RawStdEncoding.EncodeToString(hash),
		), nil
	default:
		return "", fmt.Errorf("auth: unknown hash algo %q", opts.Algo)
	}
}

// CheckPassword verifies password against an encoded hash from HashPassword
// (or a raw bcrypt hash without prefix for convenience).
func CheckPassword(encoded, password string) error {
	if strings.HasPrefix(encoded, "bcrypt$") {
		return bcrypt.CompareHashAndPassword([]byte(strings.TrimPrefix(encoded, "bcrypt$")), []byte(password))
	}
	if strings.HasPrefix(encoded, "$2a$") || strings.HasPrefix(encoded, "$2b$") || strings.HasPrefix(encoded, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password))
	}
	if strings.HasPrefix(encoded, "argon2id$") {
		return checkArgon2id(encoded, password)
	}
	return ErrInvalidCredentials
}

func checkArgon2id(encoded, password string) error {
	// argon2id$v=19$m=65536,t=1,p=4$salt$hash
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return ErrInvalidCredentials
	}
	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return ErrInvalidCredentials
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return ErrInvalidCredentials
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrInvalidCredentials
	}
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}

// HashPasswordDefault uses argon2id with default parameters.
func HashPasswordDefault(password string) (string, error) {
	return HashPassword(password, PasswordOptions{})
}

// IsInvalidCredentials reports whether err is a credential failure.
func IsInvalidCredentials(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) || errors.Is(err, bcrypt.ErrMismatchedHashAndPassword)
}
