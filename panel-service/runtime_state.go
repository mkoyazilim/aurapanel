package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

type persistedRuntimeState struct {
	State            appState            `json:"state"`
	Modules          moduleState         `json:"modules"`
	UpdateJob        panelUpdateJobState `json:"update_job,omitempty"`
	StateVersion     uint64              `json:"state_version,omitempty"`
	StateSavedAtUnix int64               `json:"state_saved_at_unix,omitempty"`
}

type runtimeSnapshot struct {
	State   appState
	Modules moduleState
}

type loadedRuntimeState struct {
	store       runtimeStateStore
	sourceIndex int
	record      runtimeStateRecord
}

var runtimeStateVersionCounter atomic.Uint64

func (s *service) loadRuntimeState() error {
	stores := runtimeStateStores()
	loaded := make([]loadedRuntimeState, 0, len(stores))
	var loadErrs []error
	for index, store := range stores {
		record, found, err := store.Load()
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %w", store.Name(), err))
			continue
		}
		if !found {
			continue
		}
		record.Payload.normalizeRuntimeStateMetadata(record.ObservedUpdatedUnix)
		loaded = append(loaded, loadedRuntimeState{
			store:       store,
			sourceIndex: index,
			record:      record,
		})
	}
	if len(loaded) == 0 {
		if len(loadErrs) > 0 {
			return errors.Join(loadErrs...)
		}
		return nil
	}
	best := loaded[0]
	for _, candidate := range loaded[1:] {
		if isRuntimeStateNewer(candidate.record.Payload, best.record.Payload) {
			best = candidate
		}
	}
	s.state = best.record.Payload.State
	s.modules = best.record.Payload.Modules
	s.updateJob = clonePanelUpdateJobState(best.record.Payload.UpdateJob)
	if s.updateJob.Running {
		// A restart cannot keep the original goroutine alive; reflect interruption instead of hanging "running".
		s.updateJob.Running = false
		if strings.TrimSpace(s.updateJob.FinishedAt) == "" {
			s.updateJob.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		}
		s.updateJob.Error = "Panel deploy interrupted by panel-service restart."
		s.updateJob.Message = "Panel deploy interrupted by service restart."
	}
	runtimeStateVersionCounter.Store(maxUint64(runtimeStateVersionCounter.Load(), best.record.Payload.StateVersion))

	if best.sourceIndex > 0 && len(stores) > 0 {
		if err := stores[0].Save(best.record.Payload); err == nil {
			log.Printf("runtime state reconciled from %s to %s", best.store.Name(), stores[0].Name())
		}
	}
	if rehydrateSeedCredentials(&s.state) {
		if err := s.saveRuntimeStateLocked(); err != nil {
			return fmt.Errorf("persist migrated runtime state: %w", err)
		}
	}
	return nil
}

func (s *service) saveRuntimeState() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveRuntimeStateLocked()
}

func (s *service) saveRuntimeStateLocked() error {
	nextVersion := runtimeStateVersionCounter.Add(1)
	savedAt := time.Now().UTC().UnixNano()
	payload := persistedRuntimeState{
		State:            s.state,
		Modules:          s.modules,
		UpdateJob:        clonePanelUpdateJobState(s.updateJob),
		StateVersion:     nextVersion,
		StateSavedAtUnix: savedAt,
	}
	stores := runtimeStateStores()
	var persistErrs []error
	for index, store := range stores {
		if err := store.Save(payload); err != nil {
			persistErrs = append(persistErrs, fmt.Errorf("%s: %w", store.Name(), err))
			continue
		}
		if index > 0 && len(persistErrs) > 0 {
			log.Printf("runtime state persist fallback used: %s", store.Name())
		}
		return nil
	}
	if len(persistErrs) > 0 {
		return errors.Join(persistErrs...)
	}
	return nil
}

func (p *persistedRuntimeState) normalizeRuntimeStateMetadata(observedUpdatedUnix int64) {
	if p.StateSavedAtUnix <= 0 && observedUpdatedUnix > 0 {
		p.StateSavedAtUnix = observedUpdatedUnix
	}
}

func isRuntimeStateNewer(candidate, current persistedRuntimeState) bool {
	if candidate.StateVersion > 0 || current.StateVersion > 0 {
		if candidate.StateVersion != current.StateVersion {
			return candidate.StateVersion > current.StateVersion
		}
	}
	if candidate.StateSavedAtUnix != current.StateSavedAtUnix {
		return candidate.StateSavedAtUnix > current.StateSavedAtUnix
	}
	return false
}

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (s *service) captureRuntimeSnapshotLocked() (runtimeSnapshot, error) {
	state, err := cloneValue(s.state)
	if err != nil {
		return runtimeSnapshot{}, fmt.Errorf("clone state: %w", err)
	}
	modules, err := cloneValue(s.modules)
	if err != nil {
		return runtimeSnapshot{}, fmt.Errorf("clone modules: %w", err)
	}
	return runtimeSnapshot{State: state, Modules: modules}, nil
}

func (s *service) restoreRuntimeSnapshotLocked(snapshot runtimeSnapshot) {
	s.state = snapshot.State
	s.modules = snapshot.Modules
}

func cloneValue[T any](input T) (T, error) {
	var zero T

	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(input); err != nil {
		return zero, err
	}

	var output T
	if err := gob.NewDecoder(&buffer).Decode(&output); err != nil {
		return zero, err
	}
	return output, nil
}

func rehydrateSeedCredentials(state *appState) bool {
	adminEmail, adminHash := loadAdminSeedCredentials()
	if strings.TrimSpace(adminHash) == "" {
		return false
	}

	changed := false
	for i := range state.Users {
		user := &state.Users[i]
		if !isSeedAdminUser(*user, adminEmail) {
			continue
		}
		if strings.TrimSpace(user.PasswordHash) == "" {
			user.PasswordHash = adminHash
			changed = true
		}
		return changed
	}

	seeded := seedState().Users[0]
	seeded.Email = adminEmail
	seeded.PasswordHash = adminHash
	seeded.ID = nextSeedUserID(*state)
	state.Users = append(state.Users, seeded)
	if state.NextUserID <= seeded.ID {
		state.NextUserID = seeded.ID + 1
	}
	return true
}

func isSeedAdminUser(user PanelUser, adminEmail string) bool {
	return strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(adminEmail)) ||
		(strings.EqualFold(strings.TrimSpace(user.Username), "admin") && strings.EqualFold(strings.TrimSpace(user.Role), "admin"))
}

func nextSeedUserID(state appState) int {
	nextID := state.NextUserID
	if nextID < 1 {
		nextID = 1
	}
	for _, user := range state.Users {
		if user.ID >= nextID {
			nextID = user.ID + 1
		}
	}
	return nextID
}

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}
