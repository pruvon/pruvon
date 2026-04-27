package ssh

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var AuthorizedKeysPath = "/home/dokku/.ssh/authorized_keys"

type SSHKey struct {
	Name    string
	KeyType string
	KeyData string
	Comment string
}

// IsValidSSHKey checks whether an SSH key is valid
func IsValidSSHKey(key string) bool {
	// Check if SSH key is empty
	if key == "" {
		return false
	}

	// SSH keys typically consist of several parts
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return false
	}

	// The first field should be the key type (ssh-rsa, ssh-ed25519, etc.)
	if !strings.HasPrefix(parts[0], "ssh-") && !strings.HasPrefix(parts[0], "ecdsa-") {
		return false
	}

	// The second field should be the key data (base64 encoded)
	// SSH key data is typically Base64 encoded and has a certain length
	if len(parts[1]) < 20 {
		return false
	}

	return true
}

// IsKeyNameExists checks whether the key name already exists
func IsKeyNameExists(name string, filepath string) (bool, error) {
	keys, err := ReadAuthorizedKeys(filepath)
	if err != nil {
		return false, err
	}

	for _, key := range keys {
		if key.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func ReadAuthorizedKeys(filepath string) ([]SSHKey, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var keys []SSHKey
	scanner := bufio.NewScanner(file)
	nameRegex := regexp.MustCompile(`NAME=([^ ]+)`)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Extract name from command string
		nameMatch := nameRegex.FindStringSubmatch(parts[0])
		if len(nameMatch) < 2 {
			continue
		}
		name := strings.Trim(nameMatch[1], "\\\"")

		// Find the key type and data
		var keyType, keyData string
		for i, part := range parts {
			if strings.HasPrefix(part, "ssh-") || strings.HasPrefix(part, "ecdsa-") {
				keyType = part
				keyData = parts[i+1]
				break
			}
		}

		comment := ""
		if len(parts) > 3 {
			comment = parts[len(parts)-1]
		}

		keys = append(keys, SSHKey{
			Name:    name,
			KeyType: keyType,
			KeyData: keyData,
			Comment: comment,
		})
	}

	return keys, scanner.Err()
}
