package service

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/samber/oops"
	"github.com/wwwangzilin/LotsACG/internal/infra/aiapi"
	"github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
	"github.com/wwwangzilin/LotsACG/internal/infra/source"
	"github.com/wwwangzilin/LotsACG/internal/infra/source/impls/pixiv"
	"github.com/wwwangzilin/LotsACG/internal/model/dto"
	"github.com/wwwangzilin/LotsACG/internal/model/query"
	"github.com/wwwangzilin/LotsACG/pkg/log"
)

// ============ XP 画像构建 (移植自 Pixiv-XP-Pusher profiler) ============

// XPStopWords 停用词表 (移植自 XP-Pusher 的 default_stop_words)。
var XPStopWords = func() map[string]struct{} {
	words := []string{
		// 通用描述
		"original", "オリジナル", "manga", "漫画", "pixiv",
		"illustration", "イラスト", "練習", "practice",
		"落書き", "doodle", "sketch", "スケッチ",
		"drawing", "art", "artwork", "fanart", "ファンアート",
		"digital", "デジタル", "アナログ", "analog",
		// 分级标签
		"r-18", "r-18g", "r18", "r18g", "nsfw", "sfw", "safe",
		// 数字/编号类
		"1000users入り", "500users入り", "100users入り", "50users入り",
		"5000users入り", "10000users入り", "users入り",
		"1000bookmarks", "500bookmarks", "100bookmarks",
		// 活动/比赛
		"コンテスト", "contest", "企画", "project",
		"お題", "リクエスト", "request", "commission",
		"落書き集", "まとめ", "詰め合わせ", "log",
		// 平台
		"twitter", "fanbox", "patreon", "skeb",
		"pixivfanbox", "fantia",
		// 通用形容词
		"cute", "kawaii", "かわいい", "可愛い",
		"beautiful", "綺麗", "pretty", "sexy",
		"cool", "かっこいい", "カッコイイ",
		// 其他无意义
		"girls", "girl", "boy", "boys", "woman", "man",
		"female", "male", "solo", "1girl", "1boy",
		"2girls", "2boys", "multiple_girls", "multiple_boys",
		"背景", "background", "風景", "landscape",
		"創作", "オリキャラ", "original_character", "oc",
		"うちの子", "看板娘", "版権", "二次創作",
		"仕事絵", "お仕事", "work",
	}
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[normalizeTag(w)] = struct{}{}
	}
	return m
}()

// isXPStopWord 判断 tag 是否为停用词。
func isXPStopWord(tag string) bool {
	_, ok := XPStopWords[normalizeTag(tag)]
	return ok
}

// XPProfileInput 是画像构建的输入作品 (从数据库历史记录)。
type XPProfileInput struct {
	Tags       []string
	CreateDate time.Time
}

// BuildXPProfileFromHistory 从数据库历史作品构建 XP 画像。
// 移植 XP-Pusher 的 TF-IDF + 时间衰减权重算法。
// 返回 {tag: weight} 按权重排序的画像。
func (s *Service) BuildXPProfileFromHistory(ctx context.Context, recentCount int) (map[string]float64, error) {
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
		return nil, oops.Wrapf(err, "failed to query artworks for xp profile")
	}

	// 统计 tag 出现次数和时间
	type occurrence struct {
		date time.Time
	}
	tagOccurrences := make(map[string][]occurrence)
	totalDocs := len(artworks)
	if totalDocs == 0 {
		return map[string]float64{}, nil
	}

	// 先统计所有 tag 的 DF (用于饱和度检测)
	tagDF := make(map[string]int)
	for _, aw := range artworks {
		if aw == nil {
			continue
		}
		createDate := aw.CreatedAt
		if createDate.IsZero() {
			createDate = time.Now()
		}
		artTagSet := make(map[string]struct{})
		for _, tag := range aw.Tags {
			if tag == nil || tag.Name == "" {
				continue
			}
			name := normalizeTag(tag.Name)
			if name == "" || isXPStopWord(name) {
				continue
			}
			artTagSet[name] = struct{}{}
			tagOccurrences[name] = append(tagOccurrences[name], occurrence{date: createDate})
		}
		for name := range artTagSet {
			tagDF[name]++
		}
	}

	// 饱和度检测: df/total > 0.5 的 tag 自动加入停用词
	const saturationThreshold = 0.5
	saturated := make(map[string]struct{})
	for tag, df := range tagDF {
		if float64(df)/float64(totalDocs) > saturationThreshold {
			saturated[tag] = struct{}{}
		}
	}

	// 计算权重: weighted_TF × IDF
	const timeDecayDays = 180.0
	profile := make(map[string]float64)
	for tag, occs := range tagOccurrences {
		if _, ok := saturated[tag]; ok {
			continue
		}
		df := tagDF[tag]
		if df == 0 {
			df = 1
		}

		// 带时间衰减的 TF
		now := time.Now()
		var weightedTF float64
		for _, occ := range occs {
			daysAgo := now.Sub(occ.date).Hours() / 24
			if daysAgo < 0 {
				daysAgo = 0
			}
			decay := math.Exp(-daysAgo / timeDecayDays)
			weightedTF += decay
		}
		// 对数抑制
		if weightedTF > 0 {
			weightedTF = math.Log10(1 + weightedTF)
		}
		// 标准 IDF (带平滑)
		idf := math.Log(float64(totalDocs)/float64(df+1)) + 1
		weight := weightedTF * idf
		if weight > 0 {
			profile[tag] = weight
		}
	}

	return profile, nil
}

// xpBookmarksCacheKey 收藏夹画像缓存 key。
func xpBookmarksCacheKey() string {
	return "xp:bookmarks_profile"
}

// BuildUserXPProfile 构建用户的完整 XP 画像:
// 1. Pixiv 收藏夹画像 (若配置了 refresh_token, 最高优先级, 移植 XP-Pusher profiler)
// 2. 用户偏好(点赞/点踩)
// 3. 群历史画像
func (s *Service) BuildUserXPProfile(ctx context.Context, userID int64) (map[string]float64, error) {
	pref, err := GetUserPreference(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile := make(map[string]float64)

	// 1. Pixiv 收藏夹画像 (最高优先级, 带缓存避免频繁拉取)
	bookmarkProfile, err := s.buildXPProfileFromBookmarks(ctx)
	if err != nil {
		log.Warn("failed to build xp profile from pixiv bookmarks, falling back", "err", err)
	} else if len(bookmarkProfile) > 0 {
		for tag, w := range bookmarkProfile {
			profile[tag] = w * 2.0
		}
		log.Info("xp profile built from pixiv bookmarks", "tags", len(bookmarkProfile))
		return profile, nil
	}

	// 2. 用户偏好权重 (第二优先级)
	for tag, w := range pref.PositiveWeights {
		if w <= 0 {
			continue
		}
		profile[tag] = w * 3.0 // 用户明确偏好加权
	}

	// 3. 群/频道历史画像 (第三优先级)
	history, err := s.BuildXPProfileFromHistory(ctx, 10000)
	if err != nil {
		log.Warn("failed to build xp profile from history", "err", err)
	} else {
		for tag, w := range history {
			if _, ok := profile[tag]; ok {
				continue // 用户偏好优先
			}
			profile[tag] = w * 0.5
		}
	}

	return profile, nil
}

// buildXPProfileFromBookmarks 通过 Pixiv OAuth 访问用户收藏夹, 构建 XP 画像。
// 移植 XP-Pusher profiler.build_profile 的 TF-IDF + 时间衰减算法。
// 结果缓存 6 小时, 避免每次推荐都重新拉取全部收藏。
func (s *Service) buildXPProfileFromBookmarks(ctx context.Context) (map[string]float64, error) {
	cfg := runtimecfg.Get().XPAIAPI.Pixiv
	if cfg.RefreshToken == "" || cfg.UserID == "" {
		return nil, oops.New("pixiv refresh_token/user_id not configured")
	}

	// 尝试读取缓存
	if cached, err := kvstor.Get[map[string]float64](ctx, xpBookmarksCacheKey()); err == nil && cached != nil {
		return cached, nil
	}

	client := pixiv.NewAppAPIClient(cfg.RefreshToken, runtimecfg.Get().Source.Proxy)
	scanLimit := runtimecfg.Get().XPAIAPI.ScanLimit
	if scanLimit <= 0 {
		scanLimit = 500
	}
	bookmarks, err := client.FetchBookmarks(ctx, cfg.UserID, scanLimit)
	if err != nil {
		return nil, oops.Wrapf(err, "failed to fetch pixiv bookmarks")
	}
	if len(bookmarks) == 0 {
		return nil, oops.New("no pixiv bookmarks found")
	}

	// 统计 tag 出现次数与时间 (与 BuildXPProfileFromHistory 相同算法)
	type occurrence struct {
		date time.Time
	}
	tagOccurrences := make(map[string][]occurrence)
	tagDF := make(map[string]int)
	totalDocs := len(bookmarks)

	for _, bm := range bookmarks {
		createDate := bm.CreateDate
		if createDate.IsZero() {
			createDate = time.Now()
		}
		artTagSet := make(map[string]struct{})
		for _, tag := range bm.Tags {
			name := normalizeTag(tag)
			if name == "" || isXPStopWord(name) {
				continue
			}
			artTagSet[name] = struct{}{}
			tagOccurrences[name] = append(tagOccurrences[name], occurrence{date: createDate})
		}
		for name := range artTagSet {
			tagDF[name]++
		}
	}

	// 饱和度检测
	const saturationThreshold = 0.5
	saturated := make(map[string]struct{})
	for tag, df := range tagDF {
		if float64(df)/float64(totalDocs) > saturationThreshold {
			saturated[tag] = struct{}{}
		}
	}

	// TF-IDF + 时间衰减
	const timeDecayDays = 180.0
	profile := make(map[string]float64)
	now := time.Now()
	for tag, occs := range tagOccurrences {
		if _, ok := saturated[tag]; ok {
			continue
		}
		df := tagDF[tag]
		if df == 0 {
			df = 1
		}
		var weightedTF float64
		for _, occ := range occs {
			daysAgo := now.Sub(occ.date).Hours() / 24
			if daysAgo < 0 {
				daysAgo = 0
			}
			weightedTF += math.Exp(-daysAgo / timeDecayDays)
		}
		if weightedTF > 0 {
			weightedTF = math.Log10(1 + weightedTF)
		}
		idf := math.Log(float64(totalDocs)/float64(df+1)) + 1
		weight := weightedTF * idf
		if weight > 0 {
			profile[tag] = weight
		}
	}

	// 缓存 6 小时
	if len(profile) > 0 {
		_ = kvstor.SetWithTTL(ctx, xpBookmarksCacheKey(), profile, 6*time.Hour)
	}
	return profile, nil
}

// ============ 组合搜索 (移植自 XP-Pusher fetcher) ============

// XPDiscover 从 Pixiv 搜索新作品:
// 1. 用 top tag pairs 组合搜索 (AND 语义)
// 2. 用 profile(收藏夹/偏好) 的 top tags 单 tag 搜索 (热门排序)
// 3. RSS 兜底
func (s *Service) XPDiscover(ctx context.Context, pref *UserPreference, profile map[string]float64, limit int) ([]*dto.FetchedArtwork, error) {
	if limit <= 0 {
		limit = 50
	}

	// 1. 组合搜索 (top pairs from 用户偏好)
	pairs := TopTagPairs(pref, 20)
	collected := make([]*dto.FetchedArtwork, 0)
	seen := make(map[string]struct{})

	addAll := func(arts []*dto.FetchedArtwork) {
		for _, art := range arts {
			if art == nil || art.SourceURL == "" {
				continue
			}
			if _, ok := seen[art.SourceURL]; ok {
				continue
			}
			seen[art.SourceURL] = struct{}{}
			collected = append(collected, art)
		}
	}

	// 组合搜索: 每个 pair 用两个 tag AND 搜索
	for _, pair := range pairs {
		if len(collected) >= limit {
			break
		}
		pairTags := []string{pair.Tag1, pair.Tag2}
		arts, err := s.SearchNewArtworksByTagsOrdered(ctx, pairTags, 10, "date_d")
		if err != nil {
			log.Debug("xp pair search failed", "pair", pair.Tag1+"+"+pair.Tag2, "err", err)
			continue
		}
		addAll(arts)
	}

	// 2. 单 tag 兜底 (优先 profile top tags, 其次用户偏好, 再次群历史)
	if len(collected) < limit {
		var topTags []TagWeight
		if len(profile) > 0 {
			topTags = TopProfileTags(profile, 15)
		}
		if len(topTags) == 0 {
			topTags = TopPreferenceTags(pref, 15)
		}
		if len(topTags) == 0 && len(collected) == 0 {
			// 完全无画像: 从群历史构建
			history, err := s.BuildXPProfileFromHistory(ctx, 10000)
			if err == nil {
				topTags = TopProfileTags(history, 15)
			}
		}
		for _, tw := range topTags {
			if len(collected) >= limit {
				break
			}
			// 权重采样: 每个 tag 单独搜索
			remaining := limit - len(collected)
			if remaining > 20 {
				remaining = 20
			}
			arts, err := s.SearchNewArtworksByTagsOrdered(ctx, []string{tw.Tag}, remaining, "popular_desc")
			if err != nil {
				continue
			}
			addAll(arts)
		}
	}

	// 3. 兜底: RSS 拉新
	if len(collected) == 0 {
		arts, err := s.FetchNewArtworks(ctx, limit)
		if err != nil {
			return nil, oops.Wrapf(err, "failed to fetch new artworks")
		}
		addAll(arts)
	}

	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected, nil
}

// SearchNewArtworksByTagsOrdered 按 tag 搜索, 支持排序 (date_d / popular_desc)。
func (s *Service) SearchNewArtworksByTagsOrdered(ctx context.Context, tags []string, limit int, order string) ([]*dto.FetchedArtwork, error) {
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
		searcher, ok := sou.(source.ArtworkTagSearcherOrdered)
		if !ok {
			continue
		}
		fetched, err := searcher.SearchArtworksByTagsOrdered(ctx, tags, limit, order)
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

// ============ 过滤排序 + AI 精排 (移植自 XP-Pusher filter + ai_scorer) ============

// XPRankedArtwork 表示一个带分数的推荐候选。
type XPRankedArtwork struct {
	Artwork *dto.FetchedArtwork
	Score   float64
}

// XPFilterAndRank 过滤并排序候选作品:
// 1. 排除已发布/已看过
// 2. 匹配度评分 (calculate_match_score)
// 3. 综合排序: match_score + 收藏数归一化
// 4. AI 精排 (若启用)
func (s *Service) XPFilterAndRank(
	ctx context.Context,
	candidates []*dto.FetchedArtwork,
	profile map[string]float64,
	pref *UserPreference,
	excludeURLs map[string]struct{},
) ([]XPRankedArtwork, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	scored := make([]XPRankedArtwork, 0, len(candidates))
	for _, art := range candidates {
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
		// 匹配度评分
		score := CalculateXPProfileMatchScore(art.Tags, profile)
		// 综合排序: 匹配度为主, 收藏数为辅
		if art.BookmarkCount > 0 {
			score = score*0.8 + math.Min(float64(art.BookmarkCount)/5000, 1.0)*0.2
		}
		scored = append(scored, XPRankedArtwork{Artwork: art, Score: score})
	}

	if len(scored) == 0 {
		return nil, nil
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Artwork.SourceURL < scored[j].Artwork.SourceURL
		}
		return scored[i].Score > scored[j].Score
	})

	// AI 精排 (可选): 对 top 候选用 LLM 打分重排
	if s.aiapi != nil && s.aiapi.Enabled() && len(scored) > 1 {
		scored = s.aiRerank(ctx, scored, profile, pref)
	}

	return scored, nil
}

// aiRerank 使用 LLM 对候选进行精排 (移植自 XP-Pusher AIScorer)。
func (s *Service) aiRerank(ctx context.Context, scored []XPRankedArtwork, profile map[string]float64, pref *UserPreference) []XPRankedArtwork {
	maxCandidates := 50
	if len(scored) > maxCandidates {
		scored = scored[:maxCandidates]
	}
	// 至少需要 3 个候选才有精排意义
	if len(scored) < 3 {
		return scored
	}

	// 构建 top tags 描述
	topTags := make([]string, 0, 10)
	for tag, w := range profile {
		_ = w
		topTags = append(topTags, tag)
		if len(topTags) >= 10 {
			break
		}
	}
	sort.Slice(topTags, func(i, j int) bool {
		return profile[topTags[i]] > profile[topTags[j]]
	})

	var recentLikes, recentDislikes []string
	if pref != nil {
		for tag := range pref.PositiveWeights {
			recentLikes = append(recentLikes, tag)
			if len(recentLikes) >= 5 {
				break
			}
		}
		for tag := range pref.NegativeWeights {
			recentDislikes = append(recentDislikes, tag)
			if len(recentDislikes) >= 5 {
				break
			}
		}
	}

	// 构建候选描述
	candidates := make([]aiapi.AICandidate, 0, len(scored))
	for _, item := range scored {
		tags := item.Artwork.Tags
		if len(tags) > 8 {
			tags = tags[:8]
		}
		candidates = append(candidates, aiapi.AICandidate{
			URL:  item.Artwork.SourceURL,
			Tags: tags,
		})
	}

	scores, err := s.aiapi.ScoreArtworks(ctx, aiRerankPrompt(topTags, recentLikes, recentDislikes, candidates), candidates)
	if err != nil {
		log.Warn("ai rerank failed, using tag scores", "err", err)
		return scored
	}

	// 混合: final = (1-w)*tag + w*ai, w=0.3
	const scoreWeight = 0.3
	for i := range scored {
		if aiScore, ok := scores[scored[i].Artwork.SourceURL]; ok {
			scored[i].Score = (1-scoreWeight)*scored[i].Score + scoreWeight*aiScore
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Artwork.SourceURL < scored[j].Artwork.SourceURL
		}
		return scored[i].Score > scored[j].Score
	})
	return scored
}

// aiRerankPrompt 构建 AI 精排 prompt (移植自 XP-Pusher AIScorer.PROMPT_TEMPLATE)。
func aiRerankPrompt(topTags, recentLikes, recentDislikes []string, candidates []aiapi.AICandidate) string {
	var sb strings.Builder
	sb.WriteString("你是推荐系统评分器。根据用户偏好, 给每个作品打\"喜爱概率\"分(0.0-1.0)。\n\n")
	sb.WriteString("用户偏好:\n")
	sb.WriteString("- 最爱 Tag: ")
	sb.WriteString(strings.Join(topTags, ", "))
	sb.WriteString("\n")
	sb.WriteString("- 最近喜欢的作品标签: ")
	sb.WriteString(strings.Join(recentLikes, ", "))
	sb.WriteString("\n")
	sb.WriteString("- 最近不喜欢的作品标签: ")
	sb.WriteString(strings.Join(recentDislikes, ", "))
	sb.WriteString("\n\n")
	sb.WriteString("候选作品:\n")
	for _, item := range candidates {
		sb.WriteString("- URL: ")
		sb.WriteString(item.URL)
		sb.WriteString(", Tags: [")
		sb.WriteString(strings.Join(item.Tags, ", "))
		sb.WriteString("]\n")
	}
	sb.WriteString("\n返回 JSON 数组, 格式: [{\"url\": \"...\", \"score\": 0.85}]\n")
	sb.WriteString("只返回 JSON, 不要解释。根据标签与用户偏好的匹配程度评分。")
	return sb.String()
}

// CalculateXPProfileMatchScore 计算作品 tag 与 XP 画像的匹配度 (0~1)。
// 移植 XP-Pusher filter.calculate_match_score。
func CalculateXPProfileMatchScore(artworkTags []string, profile map[string]float64) float64 {
	if len(artworkTags) == 0 || len(profile) == 0 {
		return 0
	}
	sortedWeights := make([]float64, 0, len(profile))
	for _, w := range profile {
		sortedWeights = append(sortedWeights, w)
	}
	sort.Slice(sortedWeights, func(i, j int) bool { return sortedWeights[i] > sortedWeights[j] })
	maxWeight := 1.0
	if len(sortedWeights) > 0 {
		maxWeight = sortedWeights[0]
	}
	topThreshold := maxWeight * 0.8
	if len(sortedWeights) >= 5 {
		topThreshold = sortedWeights[len(sortedWeights)/5]
	}

	var totalScore float64
	var matchedCount int
	var highWeightMatches int

	for _, tag := range artworkTags {
		name := normalizeTag(tag)
		if name == "" {
			continue
		}
		if weight, ok := profile[name]; ok && weight > 0 {
			totalScore += weight
			matchedCount++
			if weight >= topThreshold {
				highWeightMatches++
			}
		}
	}

	if matchedCount == 0 {
		return 0
	}
	baseScore := totalScore / (float64(matchedCount) * maxWeight)
	quantityBonus := math.Log(1+float64(matchedCount)) / math.Log(6)
	if quantityBonus > 0.3 {
		quantityBonus = 0.3
	}
	qualityBonus := float64(highWeightMatches) * 0.05
	if qualityBonus > 0.2 {
		qualityBonus = 0.2
	}
	finalScore := baseScore + quantityBonus + qualityBonus
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 1 {
		finalScore = 1
	}
	return finalScore
}

// RandomExploration 探索率: 以 discovery_rate 概率返回 true (混入探索候选)。
func RandomExploration(rate float64) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	return rand.Float64() < rate
}

// XPDiscoveryRate 返回配置的探索率。
func XPDiscoveryRate() float64 {
	cfg := runtimecfg.Get()
	return cfg.XPAIAPI.DiscoveryRate
}
