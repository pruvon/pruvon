package apps

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/pruvon/pruvon/internal/models"
)

const (
	ActionRestart = "restart"
	ActionStop    = "stop"
	ActionRebuild = "rebuild"

	RestartOperationStatePending    = "pending"
	RestartOperationStateRestarting = "restarting"
	RestartOperationStateStopping   = "stopping"
	RestartOperationStateRebuilding = "rebuilding"
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
	return StartAppActionOperation(service, ActionRestart, appName, processType)
}

func StartAppActionOperation(service *Service, action, appName, processType string) models.AppRestartOperation {
	return appRestartOperations.start(service, action, appName, processType)
}

func GetRestartOperation(id string) (models.AppRestartOperation, bool) {
	return GetAppActionOperation(id)
}

func GetAppActionOperation(id string) (models.AppRestartOperation, bool) {
	return appRestartOperations.get(id)
}

func GetLatestRestartOperation(appName string) (models.AppRestartOperation, bool) {
	return GetLatestAppActionOperation(appName)
}

func GetLatestAppActionOperation(appName string) (models.AppRestartOperation, bool) {
	return appRestartOperations.latest(appName)
}

func (s *restartOperationStore) start(service *Service, action, appName, processType string) models.AppRestartOperation {
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
		Action:      action,
		ProcessType: processType,
		State:       RestartOperationStatePending,
		Message:     actionPendingMessage(action, processType),
		StartedAt:   now,
	}

	s.operations[operation.ID] = operation
	s.latestByApp[appName] = operation.ID

	go s.run(service, operation.ID)

	return operation
}

func (s *restartOperationStore) run(service *Service, operationID string) {
	s.update(operationID, func(operation *models.AppRestartOperation) {
		operation.State = actionRunningState(operation.Action)
		operation.Message = actionRunningMessage(operation.Action, operation.ProcessType)
	})

	operation, ok := s.get(operationID)
	if !ok {
		return
	}

	if err := runAppAction(service, operation.Action, operation.AppName, operation.ProcessType); err != nil {
		s.finish(operationID, RestartOperationStateFailed, fmt.Sprintf("%s failed: %v", actionLabel(operation.Action), err))
		return
	}

	s.update(operationID, func(operation *models.AppRestartOperation) {
		operation.State = RestartOperationStateVerifying
		operation.Message = actionVerifyingMessage(operation.Action, operation.ProcessType)
	})

	if err := s.waitForActionVerification(service, operation.Action, operation.AppName, operation.ProcessType); err != nil {
		s.finish(operationID, RestartOperationStateFailed, fmt.Sprintf("%s command finished, but app status could not be confirmed: %v", actionLabel(operation.Action), err))
		return
	}

	s.finish(operationID, RestartOperationStateSucceeded, actionSucceededMessage(operation.Action, operation.ProcessType))
}

func (s *restartOperationStore) waitForActionVerification(service *Service, action, appName, processType string) error {
	deadline := s.now().Add(s.verificationTimeout)

	for {
		status, err := service.GetStatus(appName)
		if err == nil && isActionVerified(action, status, processType) {
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

func isActionVerified(action string, status *models.AppStatus, processType string) bool {
	switch action {
	case ActionStop:
		return isStopVerified(status, processType)
	case ActionRestart, ActionRebuild:
		return isRestartVerified(status, processType)
	default:
		return false
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

func isStopVerified(status *models.AppStatus, processType string) bool {
	if status == nil {
		return false
	}

	if processType != "" {
		return status.Processes[processType] == 0
	}

	switch running := status.Running.(type) {
	case bool:
		return !running
	case string:
		return running == "false"
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

func runAppAction(service *Service, action, appName, processType string) error {
	switch action {
	case ActionRestart:
		return service.RestartApp(appName, processType)
	case ActionStop:
		return service.StopApp(appName, processType)
	case ActionRebuild:
		return service.RebuildApp(appName, processType)
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}

func actionRunningState(action string) string {
	switch action {
	case ActionStop:
		return RestartOperationStateStopping
	case ActionRebuild:
		return RestartOperationStateRebuilding
	default:
		return RestartOperationStateRestarting
	}
}

func actionLabel(action string) string {
	switch action {
	case ActionStop:
		return "Stop"
	case ActionRebuild:
		return "Rebuild"
	default:
		return "Restart"
	}
}

func actionPendingMessage(action, processType string) string {
	label := actionLabel(action)
	if processType == "" {
		return fmt.Sprintf("%s request queued.", label)
	}
	return fmt.Sprintf("%s request queued for the %s process.", label, processType)
}

func actionRunningMessage(action, processType string) string {
	verb := actionPresentParticiple(action)
	if processType == "" {
		return fmt.Sprintf("Dokku is %s the application.", verb)
	}
	return fmt.Sprintf("Dokku is %s the %s process.", verb, processType)
}

func actionVerifyingMessage(action, processType string) string {
	statusLabel := "healthy"
	if action == ActionStop {
		statusLabel = "stopped"
	}
	if processType == "" {
		return fmt.Sprintf("%s command finished. Waiting for the app to report %s status.", actionLabel(action), statusLabel)
	}
	return fmt.Sprintf("%s command finished. Waiting for the %s process to report %s status.", actionLabel(action), processType, statusLabel)
}

func actionSucceededMessage(action, processType string) string {
	statusLabel := "healthy"
	if action == ActionStop {
		statusLabel = "stopped"
	}
	if processType == "" {
		return fmt.Sprintf("%s completed and the app is reporting a %s status.", actionLabel(action), statusLabel)
	}
	return fmt.Sprintf("%s completed and the %s process is reporting a %s status.", actionLabel(action), processType, statusLabel)
}

func actionPresentParticiple(action string) string {
	switch action {
	case ActionStop:
		return "stopping"
	case ActionRebuild:
		return "rebuilding"
	default:
		return "restarting"
	}
}
