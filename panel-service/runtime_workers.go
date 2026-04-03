package main

import (
	"log"
	"strconv"
	"strings"
	"time"
)

type securityStatusRateWindowState struct {
	WindowStart time.Time
	Count       int
}

func statePersistDebounce() time.Duration {
	raw := strings.TrimSpace(envOr("AURAPANEL_STATE_PERSIST_DEBOUNCE_MS", "900"))
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = 900
	}
	if value < 50 {
		value = 50
	}
	if value > 5000 {
		value = 5000
	}
	return time.Duration(value) * time.Millisecond
}

func housekeepingInterval() time.Duration {
	raw := strings.TrimSpace(envOr("AURAPANEL_HOUSEKEEPING_INTERVAL_SECONDS", "60"))
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = 60
	}
	if value < 15 {
		value = 15
	}
	if value > 600 {
		value = 600
	}
	return time.Duration(value) * time.Second
}

func securityStatusCacheTTL() time.Duration {
	raw := strings.TrimSpace(envOr("AURAPANEL_SECURITY_STATUS_CACHE_SECONDS", "8"))
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = 8
	}
	if value < 2 {
		value = 2
	}
	if value > 30 {
		value = 30
	}
	return time.Duration(value) * time.Second
}

func syncStatePersistEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(osEnv("AURAPANEL_SYNC_STATE_PERSIST"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func osEnv(key string) string {
	return strings.TrimSpace(envOr(key, ""))
}

func (s *service) enqueueStatePersist() {
	if syncStatePersistEnabled() || s.persistQueue == nil {
		if err := s.saveRuntimeState(); err != nil {
			log.Printf("runtime state save failed: %v", err)
		}
		return
	}
	select {
	case s.persistQueue <- struct{}{}:
	default:
	}
}

func (s *service) startStatePersistenceWorker() {
	if s.persistQueue == nil {
		return
	}
	debounce := s.persistDebounce
	if debounce <= 0 {
		debounce = defaultStatePersistDebounce
	}
	go func() {
		timer := time.NewTimer(time.Hour)
		if !timer.Stop() {
			<-timer.C
		}
		dirty := false
		for {
			select {
			case <-s.persistQueue:
				dirty = true
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounce)
			case <-timer.C:
				if !dirty {
					continue
				}
				if err := s.saveRuntimeState(); err != nil {
					log.Printf("runtime state save failed in worker: %v", err)
				}
				dirty = false
			}
		}
	}()
}

func (s *service) startHousekeepingWorker() {
	interval := s.housekeepingEvery
	if interval <= 0 {
		interval = defaultHousekeepingInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for now := range ticker.C {
			s.runHousekeeping(now.UTC())
		}
	}()
}

func (s *service) runHousekeeping(now time.Time) {
	cleanupServiceLoginAttempts(now)

	s.mu.Lock()
	removedWebmailTokens := 0
	for token, item := range s.modules.WebmailTokens {
		if item.ExpiresAt.Before(now) {
			delete(s.modules.WebmailTokens, token)
			removedWebmailTokens++
		}
	}
	removedDBToolTokens := 0
	for token, item := range s.modules.DBToolTokens {
		if item.ExpiresAt.Before(now) {
			delete(s.modules.DBToolTokens, token)
			removedDBToolTokens++
		}
	}

	s.cleanupExpiredDBToolAccessLocked(now)
	allowlistChanged, err := s.writeDBToolAllowlistFileLocked(now)
	s.mu.Unlock()

	if err != nil {
		log.Printf("housekeeping allowlist write failed: %v", err)
	}
	if allowlistChanged {
		s.enqueueDBToolAllowlistReload()
	}
	if removedWebmailTokens > 0 || removedDBToolTokens > 0 || allowlistChanged {
		s.enqueueStatePersist()
	}
}

func cleanupServiceLoginAttempts(now time.Time) {
	serviceLoginAttemptsMu.Lock()
	defer serviceLoginAttemptsMu.Unlock()

	for key, attempt := range serviceLoginAttempts {
		if !attempt.LockedUntil.IsZero() {
			if attempt.LockedUntil.After(now) {
				continue
			}
			delete(serviceLoginAttempts, key)
			continue
		}
		if attempt.FirstFail.IsZero() || now.Sub(attempt.FirstFail) > serviceFailureWindow {
			delete(serviceLoginAttempts, key)
		}
	}
}

func (s *service) allowSecurityStatusRequest(role, clientIP string, now time.Time) bool {
	if normalizeRole(role) == "admin" {
		return true
	}
	role = normalizeRole(role)
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		clientIP = "unknown"
	}
	key := role + "|" + clientIP

	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	if s.securityStatusRate == nil {
		s.securityStatusRate = map[string]securityStatusRateWindowState{}
	}

	expiredBefore := now.Add(-2 * securityStatusRateWindow)
	for itemKey, item := range s.securityStatusRate {
		if item.WindowStart.Before(expiredBefore) {
			delete(s.securityStatusRate, itemKey)
		}
	}

	window := s.securityStatusRate[key]
	if window.WindowStart.IsZero() || now.Sub(window.WindowStart) >= securityStatusRateWindow {
		window.WindowStart = now
		window.Count = 0
	}
	if window.Count >= securityStatusNonAdminLimit {
		s.securityStatusRate[key] = window
		return false
	}
	window.Count++
	s.securityStatusRate[key] = window
	return true
}

func (s *service) cachedSecuritySnapshot(now time.Time) securitySnapshot {
	ttl := s.securityStatusTTL
	if ttl <= 0 {
		ttl = defaultSecurityStatusCacheTTL
	}

	s.securityMu.Lock()
	cacheTime := s.securityStatusCacheTime
	cached := s.securityStatusCache
	s.securityMu.Unlock()

	if !cacheTime.IsZero() && now.Sub(cacheTime) < ttl {
		return cached
	}

	snapshot := collectSecuritySnapshot()

	s.securityMu.Lock()
	s.securityStatusCache = snapshot
	s.securityStatusCacheTime = now
	s.securityMu.Unlock()
	return snapshot
}
