package service

import (
	"math/rand"
	"testing"
)

func TestRandomPickerPick(t *testing.T) {
	picker := NewRandomPicker()
	picker.rand = rand.New(rand.NewSource(1))

	ids := []string{"u1", "u2", "u3"}
	result := picker.Pick(ids, 2)

	if len(result) != 2 {
		t.Fatalf("expected 2 ids, got %v", result)
	}
	if result[0] == result[1] {
		t.Fatalf("expected unique reviewers, got %v", result)
	}
	if !contains(ids, result[0]) || !contains(ids, result[1]) {
		t.Fatalf("unexpected reviewers %v", result)
	}
	if ids[0] != "u1" || ids[1] != "u2" || ids[2] != "u3" {
		t.Fatalf("original slice mutated: %v", ids)
	}
}

func TestRandomPickerPickOne(t *testing.T) {
	picker := NewRandomPicker()
	picker.rand = rand.New(rand.NewSource(5))

	ids := []string{"u1"}
	value, ok := picker.PickOne(ids)
	if !ok || value != "u1" {
		t.Fatalf("unexpected pick result: %v %v", value, ok)
	}

	if _, ok := picker.PickOne(nil); ok {
		t.Fatalf("expected false for empty candidates")
	}
}

func contains(list []string, candidate string) bool {
	for _, item := range list {
		if item == candidate {
			return true
		}
	}
	return false
}
