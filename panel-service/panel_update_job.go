package main

import (
	"strings"
	"time"
)

type panelUpdateJobState struct {
	Running         bool     `json:"running"`
	StartedAt       string   `json:"started_at,omitempty"`
	FinishedAt      string   `json:"finished_at,omitempty"`
	Message         string   `json:"message,omitempty"`
	Error           string   `json:"error,omitempty"`
	PreviousVersion string   `json:"previous_version,omitempty"`
	CurrentVersion  string   `json:"current_version,omitempty"`
	TargetVersion   string   `json:"target_version,omitempty"`
	Steps           []string `json:"steps,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func clonePanelUpdateJobState(src panelUpdateJobState) panelUpdateJobState {
	dst := src
	dst.Steps = append([]string{}, src.Steps...)
	dst.Warnings = append([]string{}, src.Warnings...)
	return dst
}

func (s *service) getUpdateJobSnapshot() panelUpdateJobState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clonePanelUpdateJobState(s.updateJob)
}

func (s *service) beginPanelUpdateJob() (panelUpdateJobState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.updateJob.Running {
		return clonePanelUpdateJobState(s.updateJob), false
	}

	s.updateJob = panelUpdateJobState{
		Running:       true,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Message:       "Panel deploy started.",
		TargetVersion: panelDeployRef(),
	}
	return clonePanelUpdateJobState(s.updateJob), true
}

func (s *service) runPanelUpdateJob() {
	result, err := applyPanelUpdateFromDeployScript(true)
	finishedAt := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.updateJob
	next.Running = false
	next.FinishedAt = finishedAt
	next.PreviousVersion = strings.TrimSpace(result.PreviousVersion)
	next.CurrentVersion = strings.TrimSpace(result.CurrentVersion)
	next.TargetVersion = strings.TrimSpace(result.TargetVersion)
	next.Steps = append([]string{}, result.Steps...)
	next.Warnings = append([]string{}, result.Warnings...)

	if err != nil {
		next.Error = err.Error()
		next.Message = "Panel deploy failed."
	} else {
		next.Error = ""
		next.Message = "Panel deploy completed."
		if next.CurrentVersion == "" {
			next.CurrentVersion = resolveCurrentPanelVersion()
		}
	}

	s.updateJob = next
	// Force a fresh git-based status read on next check.
	s.update = updateStatusCache{}
}
