package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type RunStatus string

const (
	RunQueued  RunStatus = "queued"
	RunRunning RunStatus = "running"
	RunPassed  RunStatus = "passed"
	RunFailed  RunStatus = "failed"
	RunErrored RunStatus = "errored"
)

type ScenarioRecord struct {
	ID        string            `json:"id"`
	Scenario  scenario.Scenario `json:"scenario"`
	CreatedAt time.Time         `json:"created_at"`
}

type RunRecord struct {
	ID         string            `json:"id"`
	ScenarioID string            `json:"scenario_id,omitempty"`
	Status     RunStatus         `json:"status"`
	Scenario   scenario.Scenario `json:"scenario"`
	Report     *report.Report    `json:"report,omitempty"`
	Error      string            `json:"error,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
}

type Memory struct {
	mu        sync.RWMutex
	scenarios map[string]ScenarioRecord
	runs      map[string]RunRecord
}

var _ Store = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		scenarios: map[string]ScenarioRecord{},
		runs:      map[string]RunRecord{},
	}
}

func (m *Memory) CreateScenario(sc scenario.Scenario) ScenarioRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	id := fmt.Sprintf("scn_%d", now.UnixNano())
	record := ScenarioRecord{ID: id, Scenario: sc, CreatedAt: now}
	m.scenarios[id] = record
	return record
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

func (m *Memory) CreateRun(scenarioID string, sc scenario.Scenario) RunRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	id := fmt.Sprintf("run_%d", now.UnixNano())
	record := RunRecord{
		ID:         id,
		ScenarioID: scenarioID,
		Status:     RunQueued,
		Scenario:   sc,
		CreatedAt:  now,
	}
	m.runs[id] = record
	return record
}

func (m *Memory) MarkRunStarted(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	record.Status = RunRunning
	record.StartedAt = &now
	m.runs[id] = record
}

func (m *Memory) CompleteRun(id string, rep report.Report) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	status := RunPassed
	if rep.Status == report.StatusFailed {
		status = RunFailed
	}
	record.Status = status
	record.Report = &rep
	record.FinishedAt = &now
	m.runs[id] = record
}

func (m *Memory) ErrorRun(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.runs[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	record.Status = RunErrored
	record.Error = err.Error()
	record.FinishedAt = &now
	m.runs[id] = record
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
