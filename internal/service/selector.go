package service

import (
	"math/rand"
	"sync"
	"time"
)

// ReviewerPicker defines selection helpers used by the repository layer.
type ReviewerPicker interface {
	Pick(ids []string, limit int) []string
	PickOne(ids []string) (string, bool)
}

// RandomPicker randomly shuffles candidates and picks deterministic subset.
type RandomPicker struct {
	mu   sync.Mutex
	rand *rand.Rand
}

// NewRandomPicker returns a picker seeded with current time.
func NewRandomPicker() *RandomPicker {
	return &RandomPicker{rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Pick returns up to "limit" unique ids randomly.
func (p *RandomPicker) Pick(ids []string, limit int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if limit <= 0 || len(ids) == 0 {
		return nil
	}

	copyIDs := append([]string(nil), ids...)
	p.rand.Shuffle(len(copyIDs), func(i, j int) {
		copyIDs[i], copyIDs[j] = copyIDs[j], copyIDs[i]
	})

	if len(copyIDs) > limit {
		copyIDs = copyIDs[:limit]
	}

	return copyIDs
}

// PickOne returns a single random id.
func (p *RandomPicker) PickOne(ids []string) (string, bool) {
	ids = p.Pick(ids, 1)
	if len(ids) == 0 {
		return "", false
	}
	return ids[0], true
}
