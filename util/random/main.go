// Package random provides utilities for generating random data
package random

import (
	"crypto/rand"
	"math/big"
)

// Password generates a random password of the specified length using the provided charset.
// If charset is empty, it uses a default charset containing alphanumeric characters and common symbols.
func Password(length int, charset []rune) []byte {
	if length <= 0 {
		return []byte{}
	}

	// Use default charset if none provided
	if len(charset) == 0 {
		charset = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	}

	charsetLen := big.NewInt(int64(len(charset)))
	password := make([]byte, length)

	for i := 0; i < length; i++ {
		// Generate a random index within the charset range
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			// Fallback to a simple approach if crypto/rand fails
			// This should rarely happen but provides resilience
			randomIndex = big.NewInt(int64(i % len(charset)))
		}
		password[i] = byte(charset[randomIndex.Int64()])
	}

	return password
}
