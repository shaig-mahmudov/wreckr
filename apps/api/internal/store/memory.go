package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type RunStatus string

const (
	RunQueued   RunStatus = "queued"
	RunRunning  RunStatus = "running"
	RunPassed   RunStatus = "passed"
	RunFailed   RunStatus = "failed"
	RunErrored  RunStatus = "errored"
	RunCanceled RunStatus = "canceled"
)

type TargetEnvironment string

const (
	TargetLocal       TargetEnvironment = "local"
	TargetDevelopment TargetEnvironment = "development"
	TargetStaging     TargetEnvironment = "staging"
	TargetProduction  TargetEnvironment = "production"
)

type TargetRecord struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	BaseURL     string            `json:"baseUrl"`
	Environment TargetEnvironment `json:"environment"`
	Description string            `json:"description,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ScenarioRecord struct {
	ID                   string            `json:"id"`
	Scenario             scenario.Scenario `json:"scenario"`
	CurrentVersionID     string            `json:"current_version_id,omitempty"`
	CurrentVersionNumber int               `json:"current_version_number,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
}

type RunRecord struct {
	ID                    string            `json:"id"`
	TargetID              string            `json:"target_id,omitempty"`
	ScenarioID            string            `json:"scenario_id,omitempty"`
	ScenarioVersionID     string            `json:"scenario_version_id,omitempty"`
	ScenarioVersionNumber int               `json:"scenario_version_number,omitempty"`
	Status                RunStatus         `json:"status"`
	Scenario              scenario.Scenario `json:"scenario"`
	Report                *report.Report    `json:"report,omitempty"`
	Error                 string            `json:"error,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
	StartedAt             *time.Time        `json:"started_at,omitempty"`
	FinishedAt            *time.Time        `json:"finished_at,omitempty"`
}

type ScenarioVersionRecord struct {
	ID            string            `json:"id"`
	ScenarioID    string            `json:"scenario_id"`
	VersionNumber int               `json:"version_number"`
	Scenario      scenario.Scenario `json:"scenario"`
	CreatedAt     time.Time         `json:"created_at"`
}

type Memory struct {
	mu               sync.RWMutex
	targets          map[string]TargetRecord
	scenarios        map[string]ScenarioRecord
	scenarioVersions map[string][]ScenarioVersionRecord
	runs             map[string]RunRecord
	runEvents        map[string][]runevent.Event
}

var _ Store = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		targets:          map[string]TargetRecord{},
		scenarios:        map[string]ScenarioRecord{},
		scenarioVersions: map[string][]ScenarioVersionRecord{},
		runs:             map[string]RunRecord{},
		runEvents:        map[string][]runevent.Event{},
	}
}

func (m *Memory) CreateTarget(target TargetRecord) TargetRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	target.ID = fmt.Sprintf("tgt_%d", now.UnixNano())
	target.CreatedAt = now
	target.UpdatedAt = now
	if target.Headers == nil {
		target.Headers = map[string]string{}
	}
	m.targets[target.ID] = target
	return target
}

func (m *Memory) UpdateTarget(id string, target TargetRecord) (TargetRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.targets[id]
	if !ok {
		return TargetRecord{}, false
	}
	target.ID = id
	target.CreatedAt = current.CreatedAt
	target.UpdatedAt = time.Now().UTC()
	if target.Headers == nil {
		target.Headers = map[string]string{}
	}
	m.targets[id] = target
	return target, true
}

func (m *Memory) DeleteTarget(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.targets[id]; !ok {
		return false
	}
	delete(m.targets, id)
	return true
}

func (m *Memory) GetTarget(id string) (TargetRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	target, ok := m.targets[id]
	return target, ok
}

func (m *Memory) ListTargets() []TargetRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]TargetRecord, 0, len(m.targets))
	for _, target := range m.targets {
		out = append(out, target)
	}
	return out
}

func (m *Memory) CreateScenario(sc scenario.Scenario) ScenarioRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	id := fmt.Sprintf("scn_%d", now.UnixNano())
	versionID := fmt.Sprintf("scv_%d", now.UnixNano())
	version := ScenarioVersionRecord{ID: versionID, ScenarioID: id, VersionNumber: 1, Scenario: sc, CreatedAt: now}
	record := ScenarioRecord{ID: id, Scenario: sc, CurrentVersionID: versionID, CurrentVersionNumber: 1, CreatedAt: now}
	m.scenarios[id] = record
	m.scenarioVersions[id] = []ScenarioVersionRecord{version}
	return record
}

func (m *Memory) UpdateScenario(id string, sc scenario.Scenario) (ScenarioRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.scenarios[id]
	if !ok {
		return ScenarioRecord{}, false
	}
	now := time.Now().UTC()
	versionNumber := len(m.scenarioVersions[id]) + 1
	versionID := fmt.Sprintf("scv_%d", now.UnixNano())
	version := ScenarioVersionRecord{ID: versionID, ScenarioID: id, VersionNumber: versionNumber, Scenario: sc, CreatedAt: now}
	m.scenarioVersions[id] = append(m.scenarioVersions[id], version)
	record.Scenario = sc
	record.CurrentVersionID = versionID
	record.CurrentVersionNumber = versionNumber
	m.scenarios[id] = record
	return record, true
}

func (m *Memory) GetScenario(id string) (ScenarioRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.scenarios[id]
	return record, ok
}

func (m *Memory) ListScenarios() []ScenarioRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ScenarioRecord, 0, len(m.scenarios))
	for _, record := range m.scenarios {
		out = append(out, record)
	}
	return out
}

func (m *Memory) ListScenarioVersions(id string) []ScenarioVersionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	versions := m.scenarioVersions[id]
	out := make([]ScenarioVersionRecord, len(versions))
	copy(out, versions)
	return out
}

func (m *Memory) CreateRun(scenarioID string, targetID string, sc scenario.Scenario) RunRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	id := fmt.Sprintf("run_%d", now.UnixNano())
	var versionID string
	var versionNumber int
	if scenarioID != "" {
		if record, ok := m.scenarios[scenarioID]; ok {
			versionID = record.CurrentVersionID
			versionNumber = record.CurrentVersionNumber
		}
	}
	record := RunRecord{
		ID:                    id,
		TargetID:              targetID,
		ScenarioID:            scenarioID,
		ScenarioVersionID:     versionID,
		ScenarioVersionNumber: versionNumber,
		Status:                RunQueued,
		Scenario:              sc,
		CreatedAt:             now,
	}
	m.runs[id] = record
	m.appendRunEventLocked(id, runevent.Event{
		Level:   runevent.LevelInfo,
		Type:    runevent.TypeRunQueued,
		Message: "run queued",
		Metadata: map[string]any{
			"scenario_id":             scenarioID,
			"scenario_version_id":     versionID,
			"scenario_version_number": versionNumber,
			"scenario":                sc.Name,
			"target_id":               targetID,
		},
	})
	return record
}

func (m *Memory) MarkRunStarted(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	if isTerminalRunStatus(record.Status) {
		return
	}
	now := time.Now().UTC()
	record.Status = RunRunning
	record.StartedAt = &now
	m.runs[id] = record
	m.appendRunEventLocked(id, runevent.Event{
		Level:   runevent.LevelInfo,
		Type:    runevent.TypeRunStarted,
		Message: "run started",
	})
}

func (m *Memory) CompleteRun(id string, rep report.Report) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	if isTerminalRunStatus(record.Status) {
		return
	}
	now := time.Now().UTC()
	status := RunPassed
	if rep.Status == report.StatusFailed {
		status = RunFailed
	}
	rep.ScenarioVersionID = record.ScenarioVersionID
	rep.ScenarioVersionNumber = record.ScenarioVersionNumber
	record.Status = status
	record.Report = &rep
	record.FinishedAt = &now
	m.runs[id] = record
	eventType := runevent.TypeRunCompleted
	level := runevent.LevelInfo
	message := "run completed"
	if status == RunFailed {
		eventType = runevent.TypeRunFailed
		level = runevent.LevelError
		message = "run failed"
	}
	m.appendRunEventLocked(id, runevent.Event{
		Level:   level,
		Type:    eventType,
		Message: message,
		Metadata: map[string]any{
			"status":          string(status),
			"total_requests":  rep.Summary.TotalRequests,
			"failed_requests": rep.Summary.FailedRequests,
			"failures":        rep.Failures,
		},
	})
}

func (m *Memory) CancelRun(id string, rep report.Report) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	if isTerminalRunStatus(record.Status) {
		return
	}
	now := time.Now().UTC()
	rep.Status = report.StatusCanceled
	rep.ScenarioVersionID = record.ScenarioVersionID
	rep.ScenarioVersionNumber = record.ScenarioVersionNumber
	record.Status = RunCanceled
	record.Report = &rep
	record.Error = "run canceled"
	record.FinishedAt = &now
	m.runs[id] = record
	m.appendRunEventLocked(id, runevent.Event{
		Level:   runevent.LevelWarn,
		Type:    runevent.TypeRunCanceled,
		Message: "run canceled",
		Metadata: map[string]any{
			"failures": rep.Failures,
		},
	})
}

func (m *Memory) ErrorRun(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	if isTerminalRunStatus(record.Status) {
		return
	}
	now := time.Now().UTC()
	record.Status = RunErrored
	record.Error = err.Error()
	record.FinishedAt = &now
	m.runs[id] = record
	m.appendRunEventLocked(id, runevent.Event{
		Level:   runevent.LevelError,
		Type:    runevent.TypeRunFailed,
		Message: err.Error(),
	})
}

func (m *Memory) GetRun(id string) (RunRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.runs[id]
	return record, ok
}

func (m *Memory) ListRuns() []RunRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]RunRecord, 0, len(m.runs))
	for _, record := range m.runs {
		out = append(out, record)
	}
	return out
}

func (m *Memory) RequestRunCancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok || !isCancelableRunStatus(record.Status) {
		return false
	}
	if m.isRunCancelRequestedLocked(id) {
		return true
	}
	m.appendRunEventLocked(id, runevent.Event{
		Level:   runevent.LevelWarn,
		Type:    runevent.TypeCancelRequested,
		Message: "run cancellation requested",
		Metadata: map[string]any{
			"status": string(record.Status),
		},
	})
	return true
}

func (m *Memory) IsRunCancelRequested(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunCancelRequestedLocked(id)
}

func (m *Memory) AppendRunEvent(runID string, event runevent.Event) runevent.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.appendRunEventLocked(runID, event)
}

func (m *Memory) ListRunEvents(runID string) []runevent.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	events := m.runEvents[runID]
	out := make([]runevent.Event, len(events))
	copy(out, events)
	return out
}

func (m *Memory) appendRunEventLocked(runID string, event runevent.Event) runevent.Event {
	now := time.Now().UTC()
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d", now.UnixNano())
	}
	if event.RunID == "" {
		event.RunID = runID
	}
	if event.Level == "" {
		event.Level = runevent.LevelInfo
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	event.Sequence = int64(len(m.runEvents[runID]) + 1)
	m.runEvents[runID] = append(m.runEvents[runID], event)
	return event
}

func isCancelableRunStatus(status RunStatus) bool {
	return status == RunQueued || status == RunRunning
}

func isTerminalRunStatus(status RunStatus) bool {
	switch status {
	case RunPassed, RunFailed, RunErrored, RunCanceled:
		return true
	default:
		return false
	}
}

func (m *Memory) isRunCancelRequestedLocked(runID string) bool {
	for _, event := range m.runEvents[runID] {
		if event.Type == runevent.TypeCancelRequested {
			return true
		}
	}
	return false
}
