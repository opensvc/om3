package random

import (
	"testing"
)

func TestPassword(t *testing.T) {
	tests := []struct {
		name     string
		length   int
		charset  []rune
		wantLen  int
		desc     string
	}{
		{
			name:    "default charset",
			length:  16,
			charset: nil,
			wantLen: 16,
			desc:    "should generate 16-character password with default charset",
		},
		{
			name:    "custom charset",
			length:  12,
			charset: []rune("ABCD1234"),
			wantLen: 12,
			desc:    "should generate 12-character password with custom charset",
		},
		{
			name:    "numeric only",
			length:  8,
			charset: []rune("0123456789"),
			wantLen: 8,
			desc:    "should generate 8-digit numeric password",
		},
		{
			name:    "zero length",
			length:  0,
			charset: []rune("ABCD"),
			wantLen: 0,
			desc:    "should return empty byte slice for zero length",
		},
		{
			name:    "negative length",
			length:  -5,
			charset: []rune("ABCD"),
			wantLen: 0,
			desc:    "should return empty byte slice for negative length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Password(tt.length, tt.charset)

			// Check length
			if len(got) != tt.wantLen {
				t.Errorf("Password() length = %d, want %d", len(got), tt.wantLen)
			}

			// Check that all characters are from the expected charset
			expectedCharset := tt.charset
			if len(expectedCharset) == 0 {
				expectedCharset = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+-=[]{}|;:,.<>?")
			}

			if len(got) > 0 {
				charsetMap := make(map[rune]bool)
				for _, r := range expectedCharset {
					charsetMap[r] = true
				}

				for _, b := range got {
					char := rune(b)
					if !charsetMap[char] {
						t.Errorf("Password() contains unexpected character: %c", char)
					}
				}
			}

			// Check that password is not empty when length > 0
			if tt.length > 0 && len(got) == 0 {
				t.Error("Password() returned empty slice for positive length")
			}

			// Check for reasonable entropy (basic check)
			if len(got) > 1 {
				hasDifferentChars := false
				for i := 1; i < len(got); i++ {
					if got[i] != got[0] {
						hasDifferentChars = true
						break
					}
				}
				if !hasDifferentChars {
					t.Error("Password() appears to have low entropy - all characters are the same")
				}
			}
		})
	}
}

func TestPasswordCharsetValidation(t *testing.T) {
	// Test that the function respects the provided charset
	tests := []struct {
		name    string
		charset []rune
		length  int
	}{
		{
			name:    "alphanumeric",
			charset: []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"),
			length:  20,
		},
		{
			name:    "hexadecimal",
			charset: []rune("0123456789ABCDEF"),
			length:  16,
		},
		{
			name:    "base64",
			charset: []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"),
			length:  24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Password(tt.length, tt.charset)

			if len(got) != tt.length {
				t.Errorf("Password() length = %d, want %d", len(got), tt.length)
			}

			// Verify all characters are from the expected charset
			charsetMap := make(map[rune]bool)
			for _, r := range tt.charset {
				charsetMap[r] = true
			}

			for _, b := range got {
				char := rune(b)
				if !charsetMap[char] {
					t.Errorf("Password() contains unexpected character: %c", char)
				}
			}
		})
	}
}