package service

import (
	"context"
	"sort"
	"strings"

	"github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
	"github.com/wwwangzilin/LotsACG/internal/infra/source"
	"github.com/wwwangzilin/LotsACG/internal/model/dto"
	"github.com/wwwangzilin/LotsACG/internal/model/entity"
	"github.com/wwwangzilin/LotsACG/internal/model/query"
	"github.com/wwwangzilin/LotsACG/pkg/log"
	"github.com/samber/oops"
)

// BuildRecentTagProfile 统计最近 recentCount 条已发布作品的 tag 出现频率,
// 作为群/频道近期内容的画像。每条已发布作品对应一条频道消息。
func (s *Service) BuildRecentTagProfile(ctx context.Context, recentCount int) map[string]float64 {
	if recentCount <= 0 {
		recentCount = 10000
	}
	artworks, err := s.QueryArtworks(ctx, query.ArtworksDB{
		ArtworksFilter: query.ArtworksFilter{
			HasPicture: true,
		},
		Paginate: query.Paginate{
			Limit: recentCount,
		},
	})
	if err != nil {
		log.Warn("failed to query recent artworks for tag profile", "err", err)
		return nil
	}

	// 忽略常见的无意义/数量 tag
	stop := map[string]struct{}{
		"r-18": {}, "r18": {}, "nsfw": {}, "original": {}, "pixiv": {},
		"illustration": {}, "イラスト": {}, "1girl": {}, "1boy": {},
	}
	freq := make(map[string]int)
	for _, aw := range artworks {
		for _, tag := range aw.Tags {
			if tag == nil || tag.Name == "" {
				continue
			}
			name := normalizeTag(tag.Name)
			if name == "" {
				continue
			}
			if _, ok := stop[name]; ok {
				continue
			}
			freq[name]++
		}
	}
	profile := make(map[string]float64, len(freq))
	for tag, count := range freq {
		profile[tag] = float64(count)
	}
	return profile
}

func normalizeTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, " ", "_")
	return tag
}

// ScoreFetchedArtworkByProfile 计算一个新拉取的作品与近期 tag 画像的匹配度。
// 匹配 tag 越多/越常见得分越高, 归一化到 0~1。
func ScoreFetchedArtworkByProfile(fetched *dto.FetchedArtwork, profile map[string]float64) float64 {
	if fetched == nil || len(profile) == 0 || len(fetched.Tags) == 0 {
		return 0
	}
	var maxWeight float64
	for _, w := range profile {
		if w > maxWeight {
			maxWeight = w
		}
	}
	if maxWeight <= 0 {
		return 0
	}
	var total float64
	matched := 0
	for _, tag := range fetched.Tags {
		if w, ok := profile[normalizeTag(tag)]; ok && w > 0 {
			total += w
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	score := total / (float64(matched) * maxWeight)
	if score > 1 {
		score = 1
	}
	return score
}

// PickBestNewArtwork 从新拉取的作品中选出一个得分最高且尚未发布的。
// 返回其 FetchedArtwork 与 sourceURL。
func (s *Service) PickBestNewArtwork(
	ctx context.Context,
	fetched []*dto.FetchedArtwork,
	profile map[string]float64,
	excludeURLs map[string]struct{},
) (*dto.FetchedArtwork, string, error) {
	var best *dto.FetchedArtwork
	bestScore := 0.0
	bestURL := ""
	for _, art := range fetched {
		if art == nil || art.SourceURL == "" || len(art.Pictures) == 0 {
			continue
		}
		if _, ok := excludeURLs[art.SourceURL]; ok {
			continue
		}
		// 已在数据库中(已发布)的作品排除
		if _, err := s.GetArtworkByURL(ctx, art.SourceURL); err == nil {
			continue
		}
		score := ScoreFetchedArtworkByProfile(art, profile)
		if score >= bestScore {
			best = art
			bestScore = score
			bestURL = art.SourceURL
		}
	}
	if best == nil {
		return nil, "", nil
	}
	return best, bestURL, nil
}

// ConvertFetchedToCached 将 FetchedArtwork 转换为可用于展示/发布的 CachedArtworkData。
func ConvertFetchedToCached(fetched *dto.FetchedArtwork) (*entity.CachedArtworkData, error) {
	if fetched == nil {
		return nil, oops.New("nil fetched artwork")
	}
	cached := &entity.CachedArtworkData{
		ID:          fetched.SourceURL,
		Title:       fetched.Title,
		Description: fetched.Description,
		R18:         fetched.R18,
		Tags:        fetched.Tags,
		SourceURL:   fetched.SourceURL,
		SourceType:  fetched.SourceType,
	}
	if fetched.Artist != nil {
		cached.Artist = &entity.CachedArtist{
			Name:     fetched.Artist.Name,
			UID:      fetched.Artist.UID,
			Username: fetched.Artist.Username,
		}
	}
	for _, p := range fetched.Pictures {
		cached.Pictures = append(cached.Pictures, &entity.CachedPicture{
			OrderIndex: p.Index,
			Thumbnail:  p.Thumbnail,
			Original:   p.Original,
			Width:      p.Width,
			Height:     p.Height,
		})
	}
	for _, u := range fetched.UgoiraMetas {
		cached.UgoiraMetas = append(cached.UgoiraMetas, &entity.CachedUgoiraMeta{
			OrderIndex: u.Index,
			MetaData:   u.Data,
		})
	}
	for _, v := range fetched.Videos {
		cached.Videos = append(cached.Videos, &entity.CachedVideo{
			OrderIndex: v.Index,
			URL:        v.URL,
			Width:      v.Width,
			Height:     v.Height,
			Duration:   v.Duration,
			Poster:     v.Poster,
			MimeType:   v.MimeType,
		})
	}
	return cached, nil
}

// TagWeight 表示一个带权重的 tag。
type TagWeight struct {
	Tag    string  `json:"tag"`
	Weight float64 `json:"weight"`
}

// TopPreferenceTags 从用户 XP 画像中取出权重最高的 n 个 tag, 按权重从高到低排序。
// 若画像为空或权重不足, 返回可用的部分。
func TopPreferenceTags(pref *UserPreference, n int) []TagWeight {
	if pref == nil || n <= 0 {
		return nil
	}
	tws := make([]TagWeight, 0, len(pref.PositiveWeights))
	for tag, w := range pref.PositiveWeights {
		if w <= 0 {
			continue
		}
		tws = append(tws, TagWeight{Tag: tag, Weight: w})
	}
	sort.Slice(tws, func(i, j int) bool {
		if tws[i].Weight == tws[j].Weight {
			return tws[i].Tag < tws[j].Tag
		}
		return tws[i].Weight > tws[j].Weight
	})
	if len(tws) > n {
		tws = tws[:n]
	}
	return tws
}

// TopProfileTags 从近期 tag 画像中取出权重最高的 n 个 tag, 按权重从高到低排序。
// 用于在没有用户 XP 画像时的兜底推荐。
func TopProfileTags(profile map[string]float64, n int) []TagWeight {
	if len(profile) == 0 || n <= 0 {
		return nil
	}
	tws := make([]TagWeight, 0, len(profile))
	for tag, w := range profile {
		if w <= 0 {
			continue
		}
		tws = append(tws, TagWeight{Tag: tag, Weight: w})
	}
	sort.Slice(tws, func(i, j int) bool {
		if tws[i].Weight == tws[j].Weight {
			return tws[i].Tag < tws[j].Tag
		}
		return tws[i].Weight > tws[j].Weight
	})
	if len(tws) > n {
		tws = tws[:n]
	}
	return tws
}

// ExpandTagsWithAI 使用 AI API 将给定的偏好 tags 扩展为更多关联/相似的搜索 tag。
// 若 AI 未启用或失败, 返回 nil。
func (s *Service) ExpandTagsWithAI(ctx context.Context, tags []string) ([]string, error) {
	if s.aiapi == nil || !s.aiapi.Enabled() {
		return nil, nil
	}
	if len(tags) == 0 {
		return nil, nil
	}
	count := runtimecfg.Get().AIAPI.RecommendTags
	if count <= 0 {
		count = 12
	}
	expanded, err := s.aiapi.ExpandTags(ctx, tags, count)
	if err != nil {
		log.Warn("ai expand tags failed", "err", err)
		return nil, err
	}
	return expanded, nil
}

// SearchNewArtworksByTags 遍历所有支持 tag 搜索的源, 按 tags 搜索新作品。
// 返回合并去重后的结果。若没有任何源支持, 返回空切片。
func (s *Service) SearchNewArtworksByTags(ctx context.Context, tags []string, limit int) ([]*dto.FetchedArtwork, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	artworks := make([]*dto.FetchedArtwork, 0)
	errs := make([]error, 0)
	seen := make(map[string]struct{})
	for _, sou := range s.sources {
		searcher, ok := sou.(source.ArtworkTagSearcher)
		if !ok {
			continue
		}
		fetched, err := searcher.SearchArtworksByTags(ctx, tags, limit)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, art := range fetched {
			if art == nil || art.SourceURL == "" {
				continue
			}
			if _, ok := seen[art.SourceURL]; ok {
				continue
			}
			seen[art.SourceURL] = struct{}{}
			artworks = append(artworks, art)
		}
	}
	if len(errs) > 0 {
		return artworks, oops.Join(errs...)
	}
	return artworks, nil
}

// ScoreFetchedArtworkByPreference 计算一个新作品与用户 XP 画像的匹配分数(0~1)。
// 使用 CalculateMatchScore, 负权重 tag 会降低分数。
func ScoreFetchedArtworkByPreference(fetched *dto.FetchedArtwork, pref *UserPreference) float64 {
	if fetched == nil || pref == nil || len(fetched.Tags) == 0 {
		return 0
	}
	return CalculateMatchScore(fetched.Tags, pref)
}

// PickBestRecommendationByXP 从新拉取的作品中, 按用户 XP 画像分数从高到低
// 选出得分最高且尚未发布/未看过的作品。
// 返回该作品、其 sourceURL 与匹配分数。
func (s *Service) PickBestRecommendationByXP(
	ctx context.Context,
	fetched []*dto.FetchedArtwork,
	pref *UserPreference,
	excludeURLs map[string]struct{},
) (*dto.FetchedArtwork, string, float64, error) {
	if len(fetched) == 0 {
		return nil, "", 0, nil
	}
	type scoredArtwork struct {
		art   *dto.FetchedArtwork
		score float64
	}
	scored := make([]scoredArtwork, 0, len(fetched))
	for _, art := range fetched {
		if art == nil || art.SourceURL == "" || len(art.Pictures) == 0 {
			continue
		}
		if _, ok := excludeURLs[art.SourceURL]; ok {
			continue
		}
		// 已在数据库中(已发布)的作品排除
		if _, err := s.GetArtworkByURL(ctx, art.SourceURL); err == nil {
			continue
		}
		score := ScoreFetchedArtworkByPreference(art, pref)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredArtwork{art: art, score: score})
	}
	if len(scored) == 0 {
		return nil, "", 0, nil
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].art.SourceURL < scored[j].art.SourceURL
		}
		return scored[i].score > scored[j].score
	})
	best := scored[0]
	return best.art, best.art.SourceURL, best.score, nil
}

