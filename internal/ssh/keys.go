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

// IsValidSSHKey SSH anahtarının geçerli olup olmadığını kontrol eder
func IsValidSSHKey(key string) bool {
	// SSH anahtarının boş olup olmadığını kontrol et
	if key == "" {
		return false
	}

	// SSH anahtarları genellikle birkaç parçadan oluşur
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return false
	}

	// İlk alan anahtar tipi olmalı (ssh-rsa, ssh-ed25519, vb.)
	if !strings.HasPrefix(parts[0], "ssh-") && !strings.HasPrefix(parts[0], "ecdsa-") {
		return false
	}

	// İkinci alan anahtar verisi olmalı (base64 ile kodlanmış)
	// SSH anahtarlarındaki veri genellikle Base64 kodludur ve belli bir uzunluğa sahiptir
	if len(parts[1]) < 20 {
		return false
	}

	return true
}

// IsKeyNameExists anahtar adının halihazırda var olup olmadığını kontrol eder
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
