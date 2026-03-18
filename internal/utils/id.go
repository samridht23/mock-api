package utils

import (
	"crypto/rand"
	"math/big"
	"time"
)

const (
	// Custom alphabet (no ambiguous characters like 0, O, I, l)
	alphabet = "123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	base     = int64(len(alphabet))
)

// NewID generates a short, unique ID (8 characters)
// Example: "aB3xK9mP"
func NewID() string {
	return GenerateID(8)
}

// NewShortID generates a very short ID (6 characters)
// Example: "k3Xm9P"
func NewShortID() string {
	return GenerateID(6)
}

// NewLongID generates a longer ID (12 characters)
// Example: "aB3xK9mPqR7s"
func NewLongID() string {
	return GenerateID(12)
}

// GenerateID generates a random ID of specified length
func GenerateID(length int) string {
	id := make([]byte, length)

	for i := 0; i < length; i++ {
		// Generate random number in range [0, len(alphabet))
		num, err := rand.Int(rand.Reader, big.NewInt(base))
		if err != nil {
			panic(err)
		}
		id[i] = alphabet[num.Int64()]
	}

	return string(id)
}

// NewTimestampID generates an ID with timestamp prefix for sortability
// Example: "1a2b3c4d-aB3xK9mP" (timestamp-random)
func NewTimestampID() string {
	timestamp := time.Now().Unix()
	timestampPart := encodeBase58(timestamp)
	randomPart := GenerateID(8)
	return timestampPart + randomPart
}

// encodeBase58 encodes a number to base58
func encodeBase58(num int64) string {
	if num == 0 {
		return string(alphabet[0])
	}

	result := ""
	for num > 0 {
		remainder := num % base
		result = string(alphabet[remainder]) + result
		num = num / base
	}

	return result
}

// IsValidID checks if an ID is valid (contains only alphabet characters)
func IsValidID(id string) bool {
	if len(id) == 0 {
		return false
	}
	for _, char := range id {
		valid := false
		for _, validChar := range alphabet {
			if char == validChar {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
	}
	return true
}
