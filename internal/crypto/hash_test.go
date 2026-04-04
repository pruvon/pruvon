package crypto

import (
	"strings"
	"testing"
)

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "Generate 8 character string",
			length: 8,
		},
		{
			name:   "Generate 16 character string",
			length: 16,
		},
		{
			name:   "Generate 32 character string",
			length: 32,
		},
		{
			name:   "Generate 64 character string",
			length: 64,
		},
		{
			name:   "Generate 1 character string",
			length: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRandomString(tt.length)

			// Check if the result has the expected length
			if len(result) != tt.length {
				t.Errorf("GenerateRandomString(%d) returned string of length %d, expected %d", tt.length, len(result), tt.length)
			}

			// Check if the result is not empty
			if result == "" && tt.length > 0 {
				t.Errorf("GenerateRandomString(%d) returned empty string", tt.length)
			}

			// Check if the result contains only valid base64 URL-safe characters
			validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
			for _, char := range result {
				if !strings.ContainsRune(validChars, char) {
					t.Errorf("GenerateRandomString(%d) returned string with invalid character: %c", tt.length, char)
				}
			}
		})
	}
}

func TestGenerateRandomStringUniqueness(t *testing.T) {
	// Generate multiple random strings and check if they are unique
	length := 16
	iterations := 100
	generated := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		result := GenerateRandomString(length)
		if generated[result] {
			t.Errorf("GenerateRandomString(%d) generated duplicate string: %s", length, result)
		}
		generated[result] = true
	}

	// We should have 100 unique strings
	if len(generated) != iterations {
		t.Errorf("GenerateRandomString(%d) generated %d unique strings, expected %d", length, len(generated), iterations)
	}
}

func TestGenerateRandomStringZeroLength(t *testing.T) {
	result := GenerateRandomString(0)
	if result != "" {
		t.Errorf("GenerateRandomString(0) should return empty string, got: %s", result)
	}
}

func BenchmarkGenerateRandomString8(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRandomString(8)
	}
}

func BenchmarkGenerateRandomString16(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRandomString(16)
	}
}

func BenchmarkGenerateRandomString32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRandomString(32)
	}
}

func BenchmarkGenerateRandomString64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRandomString(64)
	}
}
