package dokku

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pruvon/pruvon/internal/exec"
)

// OptionsReport represents the structure of Docker options for an app
type OptionsReport struct {
	Build  []string `json:"build"`
	Deploy []string `json:"deploy"`
	Run    []string `json:"run"`
}

// ParseDockerOptions parses the output of dokku docker-options:report command
// and extracts the build, deploy, and run options
func ParseDockerOptions(output string) OptionsReport {
	var result OptionsReport

	// Regex to extract options for each phase
	buildRegex := regexp.MustCompile(`Docker options build:(.*?)(?:\n|$)`)
	deployRegex := regexp.MustCompile(`Docker options deploy:(.*?)(?:\n|$)`)
	runRegex := regexp.MustCompile(`Docker options run:(.*?)(?:\n|$)`)

	// Extract and parse each type of option
	if buildMatch := buildRegex.FindStringSubmatch(output); len(buildMatch) > 1 {
		result.Build = parseOptionsList(buildMatch[1])
	}

	if deployMatch := deployRegex.FindStringSubmatch(output); len(deployMatch) > 1 {
		result.Deploy = parseOptionsList(deployMatch[1])
	}

	if runMatch := runRegex.FindStringSubmatch(output); len(runMatch) > 1 {
		result.Run = parseOptionsList(runMatch[1])
	}

	return result
}

// parseOptionsList takes a string of Docker options and splits them into individual options
// For example, "--ulimit nofile=12 --shm-size 256m" => ["--ulimit nofile=12", "--shm-size 256m"]
func parseOptionsList(optionsStr string) []string {
	var options []string

	// Trim whitespace
	trimmed := strings.TrimSpace(optionsStr)
	if trimmed == "" {
		return options
	}

	// Regex to match Docker options (handles both --option=value and --option value formats)
	// This regex pattern looks for '--' followed by text, and optionally followed by either '=value' or ' value'
	optionRegex := regexp.MustCompile(`--[a-zA-Z0-9-]+([ =][^ -][^ ]*)?`)

	matches := optionRegex.FindAllString(trimmed, -1)
	options = append(options, matches...)

	return options
}

// GetDockerOptions retrieves all Docker options for an app
func GetDockerOptions(runner exec.CommandRunner, appName string) (OptionsReport, error) {
	output, err := runner.RunCommand("dokku", "docker-options:report", appName)
	if err != nil {
		return OptionsReport{}, err
	}

	return ParseDockerOptions(output), nil
}

// AddDockerOption adds a Docker option to the specified phase (build, deploy, run)
func AddDockerOption(runner exec.CommandRunner, appName, optionType, option string) error {
	// Validate option type
	if !isValidOptionType(optionType) {
		return fmt.Errorf("invalid option type: %s", optionType)
	}

	// Run dokku command to add the option
	_, err := runner.RunCommand("dokku", "docker-options:add", appName, optionType, option)
	return err
}

// UpdateDockerOption updates a Docker option at the specified index in the given phase
// This is implemented by first removing the old option and then adding the new one
func UpdateDockerOption(runner exec.CommandRunner, appName, optionType, oldOption, newOption string) error {
	// Validate option type
	if !isValidOptionType(optionType) {
		return fmt.Errorf("invalid option type: %s", optionType)
	}

	// First delete the existing option
	_, err := runner.RunCommand("dokku", "docker-options:remove", appName, optionType, oldOption)
	if err != nil {
		return fmt.Errorf("error removing existing option: %v", err)
	}

	// Then add the new option
	_, err = runner.RunCommand("dokku", "docker-options:add", appName, optionType, newOption)
	return err
}

// DeleteDockerOption removes a Docker option from the specified phase
func DeleteDockerOption(runner exec.CommandRunner, appName, optionType, indexStr string) error {
	// Validate option type
	if !isValidOptionType(optionType) {
		return fmt.Errorf("invalid option type: %s", optionType)
	}

	// Convert index string to int
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return fmt.Errorf("invalid index: %s", indexStr)
	}

	// Get current options
	options, err := GetDockerOptions(runner, appName)
	if err != nil {
		return err
	}

	// Get the appropriate options list based on type
	var optionsList []string
	switch optionType {
	case "build":
		optionsList = options.Build
	case "deploy":
		optionsList = options.Deploy
	case "run":
		optionsList = options.Run
	}

	// Check if index is valid
	if index < 0 || index >= len(optionsList) {
		return fmt.Errorf("index out of range: %d", index)
	}

	// Get the option to remove
	optionToRemove := optionsList[index]

	// Run dokku command to remove the option
	_, err = runner.RunCommand("dokku", "docker-options:remove", appName, optionType, optionToRemove)
	return err
}

// isValidOptionType checks if the given option type is valid (build, deploy, run)
func isValidOptionType(optionType string) bool {
	return optionType == "build" || optionType == "deploy" || optionType == "run"
}
