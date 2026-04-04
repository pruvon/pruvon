package docker

import (
	"errors"
	"os"
	"testing"
)

type fakeRunner struct{ err error }

func (f fakeRunner) RunCommand(cmd string, args ...string) (string, error) { return "", f.err }
func (f fakeRunner) StartPTY(cmd string, args ...string) (*os.File, error) { return nil, nil }

func TestUpdateContainerResourceLimits_InputValidation(t *testing.T) {
	if err := UpdateContainerResourceLimits(fakeRunner{}, "cid", "", ""); err == nil {
		t.Fatal("expected error when both cpus and memory empty")
	}
}

func TestUpdateContainerResourceLimits_RunnerError(t *testing.T) {
	// Provide small valid values; we skip actual system checks by expecting runner error
	err := UpdateContainerResourceLimits(fakeRunner{err: errors.New("boom")}, "cid", "0.5", "300M")
	if err == nil {
		t.Fatal("expected runner error to propagate")
	}
}
