package fm

import (
	"sync"
	"time"
)

// TokenBucket, basit bellek içi hız sınırlayıcı (işlem/dakika).
// Üretimde site+kullanıcı bazlı sınırlar W11 API katmanında uygulanır;
// bu yapı FileService'in kancasını test etmeyi sağlar.
type TokenBucket struct {
	mu      sync.Mutex
	perMin  int
	buckets map[string][]time.Time
}

// NewTokenBucket, dakika başına işlem sınırlı limiter döndürür.
func NewTokenBucket(perMin int) *TokenBucket {
	return &TokenBucket{perMin: perMin, buckets: map[string][]time.Time{}}
}

// Allow, eylem başına sınırı denetler.
func (t *TokenBucket) Allow(action string) bool {
	if t.perMin <= 0 {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	window := now.Add(-time.Minute)

	recent := t.buckets[action][:0]
	for _, ts := range t.buckets[action] {
		if ts.After(window) {
			recent = append(recent, ts)
		}
	}
	if len(recent) >= t.perMin {
		t.buckets[action] = recent
		return false
	}
	t.buckets[action] = append(recent, now)
	return true
}
