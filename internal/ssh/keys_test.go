package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidSSHKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "Valid SSH RSA key",
			key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l user@example.com",
			expected: true,
		},
		{
			name:     "Valid SSH ED25519 key",
			key:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl user@example.com",
			expected: true,
		},
		{
			name:     "Valid ECDSA key",
			key:      "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTY user@example.com",
			expected: true,
		},
		{
			name:     "Valid SSH key without comment",
			key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l",
			expected: true,
		},
		{
			name:     "Empty string",
			key:      "",
			expected: false,
		},
		{
			name:     "Only key type without data",
			key:      "ssh-rsa",
			expected: false,
		},
		{
			name:     "Invalid key type",
			key:      "rsa-ssh AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l",
			expected: false,
		},
		{
			name:     "Key data too short",
			key:      "ssh-rsa ABC",
			expected: false,
		},
		{
			name:     "Random text",
			key:      "this is not an ssh key",
			expected: false,
		},
		{
			name:     "Single word",
			key:      "invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidSSHKey(tt.key)
			if result != tt.expected {
				t.Errorf("IsValidSSHKey(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestIsKeyNameExists(t *testing.T) {
	// Create a temporary authorized_keys file for testing
	tempDir := t.TempDir()
	authorizedKeysPath := filepath.Join(tempDir, "authorized_keys")

	// Create test data
	testContent := `command="NAME=test-key-1 /usr/local/bin/dokku" ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l user@example.com
command="NAME=test-key-2 /usr/local/bin/dokku" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl admin@example.com
`

	if err := os.WriteFile(authorizedKeysPath, []byte(testContent), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		keyName  string
		expected bool
	}{
		{
			name:     "Existing key name",
			keyName:  "test-key-1",
			expected: true,
		},
		{
			name:     "Another existing key name",
			keyName:  "test-key-2",
			expected: true,
		},
		{
			name:     "Non-existing key name",
			keyName:  "non-existing-key",
			expected: false,
		},
		{
			name:     "Empty key name",
			keyName:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsKeyNameExists(tt.keyName, authorizedKeysPath)
			if err != nil {
				t.Errorf("IsKeyNameExists(%q, %q) returned error: %v", tt.keyName, authorizedKeysPath, err)
			}
			if result != tt.expected {
				t.Errorf("IsKeyNameExists(%q, %q) = %v, expected %v", tt.keyName, authorizedKeysPath, result, tt.expected)
			}
		})
	}

	t.Run("Non-existent file", func(t *testing.T) {
		_, err := IsKeyNameExists("test-key", "/non/existent/path")
		if err == nil {
			t.Error("Expected error for non-existent file, but got nil")
		}
	})
}

func TestReadAuthorizedKeys(t *testing.T) {
	t.Run("Test with non-existent file", func(t *testing.T) {
		_, err := ReadAuthorizedKeys("/non/existent/path")
		if err == nil {
			t.Error("Expected error for non-existent file, but got nil")
		}
	})

	t.Run("Test reading valid authorized_keys file", func(t *testing.T) {
		tempDir := t.TempDir()
		authorizedKeysPath := filepath.Join(tempDir, "authorized_keys")

		testContent := `command="NAME=test-key-1 /usr/local/bin/dokku" ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l user@example.com
command="NAME=test-key-2 /usr/local/bin/dokku" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl admin@example.com
# Comment line
command="NAME=test-key-3 /usr/local/bin/dokku" ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBK8JcHd user@machine.com
`

		if err := os.WriteFile(authorizedKeysPath, []byte(testContent), 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		keys, err := ReadAuthorizedKeys(authorizedKeysPath)
		if err != nil {
			t.Errorf("ReadAuthorizedKeys returned error: %v", err)
		}

		expectedKeys := []SSHKey{
			{
				Name:    "test-key-1",
				KeyType: "ssh-rsa",
				KeyData: "AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l",
				Comment: "user@example.com",
			},
			{
				Name:    "test-key-2",
				KeyType: "ssh-ed25519",
				KeyData: "AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl",
				Comment: "admin@example.com",
			},
			{
				Name:    "test-key-3",
				KeyType: "ecdsa-sha2-nistp256",
				KeyData: "AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBK8JcHd",
				Comment: "user@machine.com",
			},
		}

		if len(keys) != len(expectedKeys) {
			t.Errorf("Expected %d keys, got %d", len(expectedKeys), len(keys))
		}

		for i, key := range keys {
			if i >= len(expectedKeys) {
				break
			}
			expected := expectedKeys[i]
			if key.Name != expected.Name {
				t.Errorf("Key %d: expected Name %q, got %q", i, expected.Name, key.Name)
			}
			if key.KeyType != expected.KeyType {
				t.Errorf("Key %d: expected KeyType %q, got %q", i, expected.KeyType, key.KeyType)
			}
			if key.KeyData != expected.KeyData {
				t.Errorf("Key %d: expected KeyData %q, got %q", i, expected.KeyData, key.KeyData)
			}
			if key.Comment != expected.Comment {
				t.Errorf("Key %d: expected Comment %q, got %q", i, expected.Comment, key.Comment)
			}
		}
	})

	t.Run("Test reading empty file", func(t *testing.T) {
		tempDir := t.TempDir()
		authorizedKeysPath := filepath.Join(tempDir, "authorized_keys")

		if err := os.WriteFile(authorizedKeysPath, []byte(""), 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		keys, err := ReadAuthorizedKeys(authorizedKeysPath)
		if err != nil {
			t.Errorf("ReadAuthorizedKeys returned error: %v", err)
		}

		if len(keys) != 0 {
			t.Errorf("Expected 0 keys from empty file, got %d", len(keys))
		}
	})

	t.Run("Test reading file with malformed lines", func(t *testing.T) {
		tempDir := t.TempDir()
		authorizedKeysPath := filepath.Join(tempDir, "authorized_keys")

		testContent := `invalid line
command="NAME=\"valid-key\" /usr/local/bin/dokku" ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l user@example.com
another invalid line
`

		if err := os.WriteFile(authorizedKeysPath, []byte(testContent), 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		keys, err := ReadAuthorizedKeys(authorizedKeysPath)
		if err != nil {
			t.Errorf("ReadAuthorizedKeys returned error: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("Expected 1 valid key, got %d", len(keys))
		}

		if len(keys) > 0 {
			if keys[0].Name != "valid-key" {
				t.Errorf("Expected key name 'valid-key', got %q", keys[0].Name)
			}
		}
	})
}

func TestSSHKeyStruct(t *testing.T) {
	t.Run("Create SSHKey struct", func(t *testing.T) {
		key := SSHKey{
			Name:    "test-key",
			KeyType: "ssh-rsa",
			KeyData: "AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l",
			Comment: "user@example.com",
		}

		if key.Name != "test-key" {
			t.Errorf("Expected Name to be 'test-key', got %s", key.Name)
		}
		if key.KeyType != "ssh-rsa" {
			t.Errorf("Expected KeyType to be 'ssh-rsa', got %s", key.KeyType)
		}
		if key.KeyData != "AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l" {
			t.Errorf("Expected KeyData to match, got %s", key.KeyData)
		}
		if key.Comment != "user@example.com" {
			t.Errorf("Expected Comment to be 'user@example.com', got %s", key.Comment)
		}
	})
}

// BenchmarkIsValidSSHKey benchmarks the SSH key validation function
func BenchmarkIsValidSSHKey(b *testing.B) {
	validKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3aWv7Z8l user@example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidSSHKey(validKey)
	}
}

// BenchmarkIsValidSSHKeyInvalid benchmarks validation with invalid key
func BenchmarkIsValidSSHKeyInvalid(b *testing.B) {
	invalidKey := "this is not a valid ssh key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidSSHKey(invalidKey)
	}
}
