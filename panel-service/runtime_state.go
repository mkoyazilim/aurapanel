package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"strings"
)

type persistedRuntimeState struct {
	State   appState    `json:"state"`
	Modules moduleState `json:"modules"`
}

type runtimeSnapshot struct {
	State   appState
	Modules moduleState
}

func (s *service) loadRuntimeState() error {
	stores := runtimeStateStores()
	var loadErrs []error
	for index, store := range stores {
		persisted, found, err := store.Load()
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %w", store.Name(), err))
			continue
		}
		if !found {
			continue
		}
		s.state = persisted.State
		s.modules = persisted.Modules
		if index > 0 && len(stores) > 0 {
			if err := stores[0].Save(persisted); err == nil {
				log.Printf("runtime state migrated from %s to %s", store.Name(), stores[0].Name())
			}
		}
		if rehydrateSeedCredentials(&s.state) {
			if err := s.saveRuntimeStateLocked(); err != nil {
				return fmt.Errorf("persist migrated runtime state: %w", err)
			}
		}
		return nil
	}
	if len(loadErrs) > 0 {
		return errors.Join(loadErrs...)
	}
	return nil
}

func (s *service) saveRuntimeState() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveRuntimeStateLocked()
}

func (s *service) saveRuntimeStateLocked() error {
	payload := persistedRuntimeState{
		State:   s.state,
		Modules: s.modules,
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
