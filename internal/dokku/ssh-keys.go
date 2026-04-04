package dokku

import (
	"encoding/json"
	"fmt"

	"github.com/pruvon/pruvon/internal/models"
)

// GetSSHKeys returns a list of SSH keys
func GetSSHKeys(runner CommandRunner) ([]models.SSHKey, error) {
	output, err := runner.RunCommand("dokku", "ssh-keys:list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("SSH keys could not be retrieved: %v", err)
	}

	var keys []models.SSHKey
	if err := json.Unmarshal([]byte(output), &keys); err != nil {
		return nil, fmt.Errorf("SSH keys could not be parsed: %v", err)
	}
	return keys, nil
}
