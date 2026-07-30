package service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
	"github.com/wwwangzilin/LotsACG/internal/model/query"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/samber/oops"
)

// UserPreference holds a per-user tag preference profile (XP profile).
// Positive weights indicate liked tags; negative weights indicate disliked tags.
type UserPreference struct {
	PositiveWeights map[string]float64 `json:"positive_weights"`
	NegativeWeights map[string]float64 `json:"negative_weights"`
}

func prefKey(userID int64) string {
	return fmt.Sprintf("preference:%d", userID)
}

func GetUserPreference(ctx context.Context, userID int64) (*UserPreference, error) {
	pref, err := kvstor.Get[*UserPreference](ctx, prefKey(userID))
	if err != nil {
		return &UserPreference{
			PositiveWeights: make(map[string]float64),
			NegativeWeights: make(map[string]float64),
		}, nil
	}
	if pref.PositiveWeights == nil {
		pref.PositiveWeights = make(map[string]float64)
	}
	if pref.NegativeWeights == nil {
		pref.NegativeWeights = make(map[string]float64)
	}
	return pref, nil
}

func SaveUserPreference(ctx context.Context, userID int64, pref *UserPreference) error {
	return kvstor.SetWithTTL(ctx, prefKey(userID), pref, 30*24*time.Hour)
}

// UpdatePreferenceFromLike boosts tag weights for liked artwork tags.
// Uses a TF-IDF-inspired formula: each occurrence adds weight, with log saturation.
func UpdatePreferenceFromLike(ctx context.Context, userID int64, tags []string) error {
	pref, err := GetUserPreference(ctx, userID)
	if err != nil {
		return err
	}
	const likeBoost = 1.0
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		pref.PositiveWeights[tag] += likeBoost
	}
	return SaveUserPreference(ctx, userID, pref)
}

// UpdatePreferenceFromDislike penalizes tag weights for disliked artwork tags.
// Also records them in the negative profile so similar works are ranked lower.
func UpdatePreferenceFromDislike(ctx context.Context, userID int64, tags []string) error {
	pref, err := GetUserPreference(ctx, userID)
	if err != nil {
		return err
	}
	const dislikePenalty = 0.5
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		pref.PositiveWeights[tag] -= dislikePenalty
		// Clamp positive weights to avoid going too negative
		if pref.PositiveWeights[tag] < 0 {
			pref.PositiveWeights[tag] = 0
		}
		pref.NegativeWeights[tag] += dislikePenalty
	}
	return SaveUserPreference(ctx, userID, pref)
}

// CalculateMatchScore computes how well a set of artwork tags matches the user's preference.
// Based on the algorithm from Pixiv-XP-Pusher:
//
//	baseScore = sum(matched weights) / (matchedCount * maxWeight)
//	+ quantityBonus: log(1+matchedCount) / log(6)  (cap at 0.3)
//	+ qualityBonus: min(highWeightMatches * 0.05, 0.2)
//	- negativePenalty
//
// Result is clamped to [0, 1].
func CalculateMatchScore(artworkTags []string, pref *UserPreference) float64 {
	if len(artworkTags) == 0 || pref == nil {
		return 0
	}
	if len(pref.PositiveWeights) == 0 && len(pref.NegativeWeights) == 0 {
		return 0
	}

	// Find max weight in positive profile
	var maxWeight float64
	var topThreshold float64
	sortedWeights := make([]float64, 0, len(pref.PositiveWeights))
	for _, w := range pref.PositiveWeights {
		sortedWeights = append(sortedWeights, w)
	}
	sort.Slice(sortedWeights, func(i, j int) bool { return sortedWeights[i] > sortedWeights[j] })
	if len(sortedWeights) > 0 {
		maxWeight = sortedWeights[0]
		topIdx := len(sortedWeights) / 5
		if topIdx >= len(sortedWeights) {
			topIdx = len(sortedWeights) - 1
		}
		topThreshold = sortedWeights[topIdx]
	} else {
		maxWeight = 1.0
		topThreshold = 0.8
	}

	var totalScore float64
	var matchedCount int
	var highWeightMatches int
	var negativePenalty float64

	for _, tag := range artworkTags {
		if tag == "" {
			continue
		}
		// Positive match
		if weight, ok := pref.PositiveWeights[tag]; ok && weight > 0 {
			totalScore += weight
			matchedCount++
			if weight >= topThreshold {
				highWeightMatches++
			}
		}
		// Negative match
		if negWeight, ok := pref.NegativeWeights[tag]; ok && negWeight > 0 {
			negativePenalty += negWeight * 0.5
		}
	}

	if matchedCount == 0 {
		if negativePenalty > 0 {
			pen := negativePenalty / (maxWeight + 1)
			if pen > 0.5 {
				pen = 0.5
			}
			return 0
		}
		return 0
	}

	// Base: weight sum / (matchedCount * maxWeight)
	baseScore := totalScore / (float64(matchedCount) * maxWeight)

	// Quantity bonus: log(1+n) / log(6), cap at 0.3
	quantityBonus := math.Log(1+float64(matchedCount)) / math.Log(6)
	if quantityBonus > 0.3 {
		quantityBonus = 0.3
	}

	// Quality bonus: min(highWeightMatches * 0.05, 0.2)
	qualityBonus := float64(highWeightMatches) * 0.05
	if qualityBonus > 0.2 {
		qualityBonus = 0.2
	}

	// Negative penalty
	penaltyNormalized := negativePenalty / (maxWeight + 1)

	finalScore := baseScore + quantityBonus + qualityBonus - penaltyNormalized
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 1 {
		finalScore = 1
	}
	return finalScore
}

// RankArtworksByPreference scores a list of artwork tag sets against the user preference
// and returns them sorted descending by score.
func RankArtworksByPreference[T any](
	items []T,
	getTags func(T) []string,
	pref *UserPreference,
) []struct {
	Item  T
	Score float64
} {
	type scored struct {
		Item  T
		Score float64
	}
	scoredItems := make([]scored, 0, len(items))
	for _, item := range items {
		tags := getTags(item)
		score := CalculateMatchScore(tags, pref)
		scoredItems = append(scoredItems, scored{Item: item, Score: score})
	}

	// Apply small random noise for variety (shuffle factor)
	const shuffleFactor = 0.15
	for i := range scoredItems {
		noise := rand.Float64()*shuffleFactor*2 - shuffleFactor
		scoredItems[i].Score += noise
	}

	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].Score > scoredItems[j].Score
	})

	return scoredItems
}

// PickBestRecommendation picks the best artwork from a batch of candidates
// using the user's preference profile. Skips seen artworks.
// Returns the highest-scoring unseen artwork (with diversity consideration).
func PickBestRecommendation(
	artworks []shared.ArtworkLike,
	seenURLs []string,
	pref *UserPreference,
) shared.ArtworkLike {
	if len(artworks) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(seenURLs))
	for _, u := range seenURLs {
		seen[u] = struct{}{}
	}

	filtered := make([]shared.ArtworkLike, 0, len(artworks))
	for _, aw := range artworks {
		if _, ok := seen[aw.GetSourceURL()]; ok {
			continue
		}
		filtered = append(filtered, aw)
	}
	if len(filtered) == 0 {
		return nil
	}

	scored := RankArtworksByPreference(filtered,
		func(aw shared.ArtworkLike) []string { return aw.GetTags() },
		pref)
	if len(scored) == 0 {
		return nil
	}
	return scored[0].Item
}

// FetchAndScoreBatch fetches a batch of random artworks and returns the top scored one.
func (s *Service) FetchAndScoreRecommendations(
	ctx context.Context,
	userID int64,
	batchSize int,
	seenURLs []string,
) (shared.ArtworkLike, error) {
	pref, err := GetUserPreference(ctx, userID)
	if err != nil {
		return nil, oops.Wrapf(err, "failed to get user preference")
	}

	aw, err := s.QueryArtworks(ctx, query.ArtworksDB{
		ArtworksFilter: query.ArtworksFilter{
			HasPicture: true,
		},
		Paginate: query.Paginate{
			Offset: 0,
			Limit:  batchSize,
		},
		Random: true,
	})
	if err != nil {
		return nil, oops.Wrapf(err, "failed to query artworks")
	}
	if len(aw) == 0 {
		return nil, nil
	}

	artworkLikes := make([]shared.ArtworkLike, len(aw))
	for i, a := range aw {
		artworkLikes[i] = a
	}

	best := PickBestRecommendation(artworkLikes, seenURLs, pref)
	return best, nil
}
