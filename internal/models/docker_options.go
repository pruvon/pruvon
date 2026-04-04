package models

// DockerOption represents a Docker option for Dokku applications
type DockerOption struct {
	Type  string `json:"type"`   // build, deploy, or run
	Value string `json:"option"` // The Docker option value (e.g., --memory 512m)
}
