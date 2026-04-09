package apps

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/pruvon/pruvon/internal/models"
)

const (
	RestartOperationStatePending    = "pending"
	RestartOperationStateRestarting = "restarting"
	RestartOperationStateVerifying  = "verifying"
	RestartOperationStateSucceeded  = "succeeded"
	RestartOperationStateFailed     = "failed"

	restartOperationTTL                = 15 * time.Minute
	defaultRestartVerificationTimeout  = 45 * time.Second
	defaultRestartVerificationInterval = 2 * time.Second
)

type restartOperationStore struct {
	mu          sync.RWMutex
	operations  map[string]models.AppRestartOperation
	latestByApp map[string]string
	now         func() time.Time

	verificationTimeout  time.Duration
	verificationInterval time.Duration
}

var appRestartOperations = newRestartOperationStore()

func newRestartOperationStore() *restartOperationStore {
	return &restartOperationStore{
		operations:           make(map[string]models.AppRestartOperation),
		latestByApp:          make(map[string]string),
		now:                  time.Now,
		verificationTimeout:  defaultRestartVerificationTimeout,
		verificationInterval: defaultRestartVerificationInterval,
	}
}

func StartRestartOperation(service *Service, appName, processType string) models.AppRestartOperation {
	return appRestartOperations.start(service, appName, processType)
}

func GetRestartOperation(id string) (models.AppRestartOperation, bool) {
	return appRestartOperations.get(id)
}

func GetLatestRestartOperation(appName string) (models.AppRestartOperation, bool) {
	return appRestartOperations.latest(appName)
}

func (s *restartOperationStore) start(service *Service, appName, processType string) models.AppRestartOperation {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	if current, ok := s.activeForAppLocked(appName); ok {
		current.Reused = true
		return current
	}

	now := s.now().UTC()
	operation := models.AppRestartOperation{
		ID:          uuid.NewString(),
		AppName:     appName,
		Action:      "restart",
		ProcessType: processType,
		State:       RestartOperationStatePending,
		Message:     restartPendingMessage(processType),
		StartedAt:   now,
	}

	s.operations[operation.ID] = operation
	s.latestByApp[appName] = operation.ID

	go s.run(service, operation.ID)

	return operation
}

func (s *restartOperationStore) run(service *Service, operationID string) {
	s.update(operationID, func(operation *models.AppRestartOperation) {
		operation.State = RestartOperationStateRestarting
		operation.Message = restartRunningMessage(operation.ProcessType)
	})

	operation, ok := s.get(operationID)
	if !ok {
		return
	}

	if err := service.RestartApp(operation.AppName, operation.ProcessType); err != nil {
		s.finish(operationID, RestartOperationStateFailed, fmt.Sprintf("Restart failed: %v", err))
		return
	}

	s.update(operationID, func(operation *models.AppRestartOperation) {
		operation.State = RestartOperationStateVerifying
		operation.Message = restartVerifyingMessage(operation.ProcessType)
	})

	if err := s.waitForRestartVerification(service, operation.AppName, operation.ProcessType); err != nil {
		s.finish(operationID, RestartOperationStateFailed, fmt.Sprintf("Restart command finished, but app status could not be confirmed: %v", err))
		return
	}

	s.finish(operationID, RestartOperationStateSucceeded, restartSucceededMessage(operation.ProcessType))
}

func (s *restartOperationStore) waitForRestartVerification(service *Service, appName, processType string) error {
	deadline := s.now().Add(s.verificationTimeout)

	for {
		status, err := service.GetStatus(appName)
		if err == nil && isRestartVerified(status, processType) {
			return nil
		}

		if err != nil {
			exists, existsErr := service.AppExists(appName)
			if existsErr == nil && !exists {
				return fmt.Errorf("app %q no longer exists", appName)
			}
		}

		if s.now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("timed out after %s", s.verificationTimeout)
		}

		time.Sleep(s.verificationInterval)
	}
}

func isRestartVerified(status *models.AppStatus, processType string) bool {
	if status == nil || !status.Deployed {
		return false
	}

	if processType != "" {
		return status.Processes[processType] > 0
	}

	switch running := status.Running.(type) {
	case bool:
		return running
	case string:
		return running == "true" || running == "mixed"
	default:
		return false
	}
}

func (s *restartOperationStore) finish(operationID, state, message string) {
	finishedAt := s.now().UTC()
	s.update(operationID, func(operation *models.AppRestartOperation) {
		operation.State = state
		operation.Message = message
		operation.FinishedAt = &finishedAt
	})
}

func (s *restartOperationStore) get(id string) (models.AppRestartOperation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	operation, ok := s.operations[id]
	return operation, ok
}

func (s *restartOperationStore) latest(appName string) (models.AppRestartOperation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	operationID, ok := s.latestByApp[appName]
	if !ok {
		return models.AppRestartOperation{}, false
	}

	operation, ok := s.operations[operationID]
	if !ok {
		delete(s.latestByApp, appName)
		return models.AppRestartOperation{}, false
	}

	return operation, true
}

func (s *restartOperationStore) update(operationID string, fn func(*models.AppRestartOperation)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	operation, ok := s.operations[operationID]
	if !ok {
		return
	}

	fn(&operation)
	s.operations[operationID] = operation
	if operation.AppName != "" {
		s.latestByApp[operation.AppName] = operationID
	}
}

func (s *restartOperationStore) activeForAppLocked(appName string) (models.AppRestartOperation, bool) {
	operationID, ok := s.latestByApp[appName]
	if !ok {
		return models.AppRestartOperation{}, false
	}

	operation, ok := s.operations[operationID]
	if !ok {
		delete(s.latestByApp, appName)
		return models.AppRestartOperation{}, false
	}

	if isRestartTerminalState(operation.State) {
		return models.AppRestartOperation{}, false
	}

	return operation, true
}

func (s *restartOperationStore) cleanupExpiredLocked() {
	now := s.now()
	for id, operation := range s.operations {
		if !isRestartTerminalState(operation.State) || operation.FinishedAt == nil {
			continue
		}

		if now.Sub(*operation.FinishedAt) <= restartOperationTTL {
			continue
		}

		delete(s.operations, id)
		if s.latestByApp[operation.AppName] == id {
			delete(s.latestByApp, operation.AppName)
		}
	}
}

func isRestartTerminalState(state string) bool {
	return state == RestartOperationStateSucceeded || state == RestartOperationStateFailed
}

func restartPendingMessage(processType string) string {
	if processType == "" {
		return "Restart request queued."
	}
	return fmt.Sprintf("Restart request queued for the %s process.", processType)
}

func restartRunningMessage(processType string) string {
	if processType == "" {
		return "Dokku is restarting the application."
	}
	return fmt.Sprintf("Dokku is restarting the %s process.", processType)
}

func restartVerifyingMessage(processType string) string {
	if processType == "" {
		return "Restart command finished. Waiting for the app to report healthy status."
	}
	return fmt.Sprintf("Restart command finished. Waiting for the %s process to report healthy status.", processType)
}

func restartSucceededMessage(processType string) string {
	if processType == "" {
		return "Restart completed and the app is reporting a healthy status."
	}
	return fmt.Sprintf("Restart completed and the %s process is reporting a healthy status.", processType)
}
