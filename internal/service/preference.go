package service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/samber/oops"
	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
	"github.com/wwwangzilin/LotsACG/internal/model/query"
	"github.com/wwwangzilin/LotsACG/internal/shared"
)

// UserPreference holds a per-user tag preference profile (XP profile).
// Positive weights indicate liked tags; negative weights indicate disliked tags.
type UserPreference struct {
	PositiveWeights map[string]float64 `json:"positive_weights"`
	NegativeWeights map[string]float64 `json:"negative_weights"`
	// TagUpdatedAt 记录每个正向 tag 最近一次更新时间 (用于时间衰减)
	TagUpdatedAt map[string]time.Time `json:"tag_updated_at,omitempty"`
	// TagPairs 记录 tag 组合权重 (用于组合搜索), key 为 "tag1|tag2"
	TagPairs map[string]float64 `json:"tag_pairs,omitempty"`
}

func prefKey(userID int64) string {
	return fmt.Sprintf("preference:%d", userID)
}

func GetUserPreference(ctx context.Context, userID int64) (*UserPreference, error) {
	pref, err := kvstor.Get[*UserPreference](ctx, prefKey(userID))
	if err != nil {
		return newEmptyPreference(), nil
	}
	if pref.PositiveWeights == nil {
		pref.PositiveWeights = make(map[string]float64)
	}
	if pref.NegativeWeights == nil {
		pref.NegativeWeights = make(map[string]float64)
	}
	if pref.TagUpdatedAt == nil {
		pref.TagUpdatedAt = make(map[string]time.Time)
	}
	if pref.TagPairs == nil {
		pref.TagPairs = make(map[string]float64)
	}
	return pref, nil
}

func newEmptyPreference() *UserPreference {
	return &UserPreference{
		PositiveWeights: make(map[string]float64),
		NegativeWeights: make(map[string]float64),
		TagUpdatedAt:    make(map[string]time.Time),
		TagPairs:        make(map[string]float64),
	}
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
	now := time.Now()
	for _, tag := range tags {
		name := normalizeTag(tag)
		if name == "" {
			continue
		}
		pref.PositiveWeights[name] += likeBoost
		pref.TagUpdatedAt[name] = now
	}
	updateTagPairs(pref, tags, 1.0)
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
		name := normalizeTag(tag)
		if name == "" {
			continue
		}
		pref.PositiveWeights[name] -= dislikePenalty
		// Clamp positive weights to avoid going too negative
		if pref.PositiveWeights[name] < 0 {
			pref.PositiveWeights[name] = 0
		}
		pref.NegativeWeights[name] += dislikePenalty
	}
	return SaveUserPreference(ctx, userID, pref)
}

// updateTagPairs 更新 tag 组合权重 (共现计数)。
// 同一作品中出现的 tag 两两组合, 权重递增。
func updateTagPairs(pref *UserPreference, tags []string, delta float64) {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		name := normalizeTag(tag)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) < 2 {
		return
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			if a > b {
				a, b = b, a
			}
			key := a + "|" + b
			pref.TagPairs[key] += delta
		}
	}
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
		name := normalizeTag(tag)
		// Positive match
		if weight, ok := pref.PositiveWeights[name]; ok && weight > 0 {
			totalScore += weight
			matchedCount++
			if weight >= topThreshold {
				highWeightMatches++
			}
		}
		// Negative match
		if negWeight, ok := pref.NegativeWeights[name]; ok && negWeight > 0 {
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
	scoredItems := make([]struct {
		Item  T
		Score float64
	}, 0, len(items))
	for _, item := range items {
		tags := getTags(item)
		score := CalculateMatchScore(tags, pref)
		scoredItems = append(scoredItems, struct {
			Item  T
			Score float64
		}{Item: item, Score: score})
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

// TagPair 表示一个 tag 组合及其权重。
type TagPair struct {
	Tag1   string  `json:"tag1"`
	Tag2   string  `json:"tag2"`
	Weight float64 `json:"weight"`
}

// TopTagPairs 返回权重最高的 n 个 tag 组合 (按权重降序)。
// 用于 XP 组合搜索: 同时包含这两个 tag 的作品更符合用户偏好。
func TopTagPairs(pref *UserPreference, n int) []TagPair {
	if pref == nil || n <= 0 {
		return nil
	}
	pairs := make([]TagPair, 0, len(pref.TagPairs))
	for key, w := range pref.TagPairs {
		if w <= 0 {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		pairs = append(pairs, TagPair{Tag1: parts[0], Tag2: parts[1], Weight: w})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Weight == pairs[j].Weight {
			return pairs[i].Tag1+pairs[i].Tag2 < pairs[j].Tag1+pairs[j].Tag2
		}
		return pairs[i].Weight > pairs[j].Weight
	})
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	return pairs
}
