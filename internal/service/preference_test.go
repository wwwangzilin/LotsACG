package service

import (
	"testing"
)

func TestCalculateMatchScore_Empty(t *testing.T) {
	pref := &UserPreference{
		PositiveWeights: map[string]float64{},
		NegativeWeights: map[string]float64{},
	}
	score := CalculateMatchScore([]string{"cat", "dog"}, pref)
	if score != 0 {
		t.Fatalf("expected 0 for empty preference, got %f", score)
	}
}

func TestCalculateMatchScore_ExactMatch(t *testing.T) {
	pref := &UserPreference{
		PositiveWeights: map[string]float64{
			"cat":  5.0,
			"dog":  3.0,
			"bird": 1.0,
		},
		NegativeWeights: map[string]float64{},
	}
	score := CalculateMatchScore([]string{"cat", "dog"}, pref)
	if score <= 0 {
		t.Fatalf("expected positive score for matching tags, got %f", score)
	}
}

func TestCalculateMatchScore_NegativePenalty(t *testing.T) {
	pref := &UserPreference{
		PositiveWeights: map[string]float64{
			"cat": 5.0,
		},
		NegativeWeights: map[string]float64{
			"dog": 2.0,
		},
	}
	scoreCat := CalculateMatchScore([]string{"cat"}, pref)
	scoreCatDog := CalculateMatchScore([]string{"cat", "dog"}, pref)
	if scoreCatDog >= scoreCat {
		t.Fatalf("expected score to decrease when disliked tag present, before=%f after=%f", scoreCat, scoreCatDog)
	}
}

func TestCalculateMatchScore_NoMatch(t *testing.T) {
	pref := &UserPreference{
		PositiveWeights: map[string]float64{
			"cat": 5.0,
		},
		NegativeWeights: map[string]float64{},
	}
	score := CalculateMatchScore([]string{"unknown_tag"}, pref)
	if score != 0 {
		t.Fatalf("expected 0 for no match, got %f", score)
	}
}

func TestCalculateMatchScore_ColdStart(t *testing.T) {
	pref := &UserPreference{
		PositiveWeights: map[string]float64{
			"cat": 2.0,
		},
		NegativeWeights: map[string]float64{},
	}
	// Cold start: few likes, many candidate tags
	score := CalculateMatchScore([]string{"cat", "dog", "bird", "fish"}, pref)
	if score <= 0 {
		t.Fatalf("expected positive score when at least one tag matches, got %f", score)
	}
}
