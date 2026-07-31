package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/samber/oops"
	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/metautil"
	"github.com/wwwangzilin/LotsACG/internal/model/entity"
	"github.com/wwwangzilin/LotsACG/internal/service"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/wwwangzilin/LotsACG/pkg/log"
)

type recommendationSession struct {
	LikedSourceURLs  []string `json:"liked_source_urls"`
	SeenSourceURLs   []string `json:"seen_source_urls"`
	CurrentSourceURL string   `json:"current_source_url"`
	CurrentTags      []string `json:"current_tags"`
}

func Recommend(ctx *telegohandler.Context, message telego.Message) error {
	if message.Chat.Type != telego.ChatTypePrivate {
		utils.ReplyMessage(ctx, message, "这个功能仅支持在私聊中使用")
		return nil
	}
	serv, err := requireService(ctx)
	if err != nil {
		return err
	}
	meta, err := requireMeta(ctx)
	if err != nil {
		return err
	}
	return sendRecommendation(ctx, ctx, message.Chat.ChatID(), message.From.ID, serv, meta, message.MessageID)
}

func RecommendCallbackQuery(ctx *telegohandler.Context, query telego.CallbackQuery) error {
	serv, err := requireService(ctx)
	if err != nil {
		return err
	}
	meta, err := requireMeta(ctx)
	if err != nil {
		return err
	}
	if query.Message == nil || query.Message.GetChat().Type != telego.ChatTypePrivate {
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "请在私聊中使用", ShowAlert: true, CacheTime: 60})
		return nil
	}

	session, err := loadRecommendationSession(ctx, query.From.ID)
	if err != nil {
		session = &recommendationSession{}
	}
	var action string
	if len(query.Data) > 0 {
		action = query.Data
	}

	switch action {
	case "recommend_like":
		_ = updatePreferenceFromSession(ctx, serv, query.From.ID, session, true)
		if session.CurrentSourceURL != "" {
			session.addLike(session.CurrentSourceURL)
			_ = saveRecommendationSession(ctx, query.From.ID, session)
		}
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "已加入收藏，已记住你的偏好", CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_dislike":
		_ = updatePreferenceFromSession(ctx, serv, query.From.ID, session, false)
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "已记录，会减少这类推荐", CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_next":
		if session.CurrentSourceURL != "" {
			session.addSeen(session.CurrentSourceURL)
			_ = saveRecommendationSession(ctx, query.From.ID, session)
		}
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "为你换一个推荐", CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_push":
		if len(session.LikedSourceURLs) == 0 {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "还没有收藏任何作品", ShowAlert: true, CacheTime: 30})
			return nil
		}
		count, err := pushRecommendationSelection(ctx, ctx, serv, meta, query.Message.GetChat().ChatID(), 0, session.LikedSourceURLs)
		if err != nil {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "推送失败: " + err.Error(), ShowAlert: true, CacheTime: 30})
			return nil
		}
		session.LikedSourceURLs = nil
		_ = saveRecommendationSession(ctx, query.From.ID, session)
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: fmt.Sprintf("已推送 %d 个作品到频道", count), CacheTime: 10})
		return nil
	default:
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "无效操作", CacheTime: 10})
		return nil
	}
}

func loadRecommendationSession(ctx context.Context, userID int64) (*recommendationSession, error) {
	key := recommendSessionKey(userID)
	session, err := kvstor.Get[recommendationSession](ctx, key)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func saveRecommendationSession(ctx context.Context, userID int64, session *recommendationSession) error {
	key := recommendSessionKey(userID)
	return kvstor.SetWithTTL(ctx, key, session, 7*24*time.Hour)
}

func recommendSessionKey(userID int64) string {
	return fmt.Sprintf("recommend:%d", userID)
}

func (s *recommendationSession) addLike(sourceURL string) {
	if sourceURL == "" {
		return
	}
	for _, item := range s.LikedSourceURLs {
		if item == sourceURL {
			return
		}
	}
	s.LikedSourceURLs = append(s.LikedSourceURLs, sourceURL)
}

func (s *recommendationSession) addSeen(sourceURL string) {
	if sourceURL == "" {
		return
	}
	for _, item := range s.SeenSourceURLs {
		if item == sourceURL {
			return
		}
	}
	s.SeenSourceURLs = append(s.SeenSourceURLs, sourceURL)
	if len(s.SeenSourceURLs) > 20 {
		s.SeenSourceURLs = s.SeenSourceURLs[len(s.SeenSourceURLs)-20:]
	}
}

// updatePreferenceFromSession updates the user's preference profile from the current artwork.
// isLike=true boosts tags, isLike=false penalizes tags.
func updatePreferenceFromSession(ctx context.Context, serv *service.Service, userID int64, session *recommendationSession, isLike bool) error {
	if session.CurrentSourceURL == "" {
		return nil
	}
	var tags []string
	if len(session.CurrentTags) > 0 {
		// 新图尚未入库, 使用会话中保存的 tags
		tags = session.CurrentTags
	} else {
		awEnt, err := serv.GetArtworkByURL(ctx, session.CurrentSourceURL)
		if err != nil {
			return nil
		}
		tags = extractTagNames(awEnt.Tags)
	}
	if len(tags) == 0 {
		return nil
	}
	if isLike {
		return service.UpdatePreferenceFromLike(ctx, userID, tags)
	}
	return service.UpdatePreferenceFromDislike(ctx, userID, tags)
}

func extractTagNames(tags []*entity.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != nil && t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names
}

func sendRecommendation(ctx context.Context, tgCtx *telegohandler.Context, chatID telego.ChatID, userID int64, serv *service.Service, meta *metautil.MetaData, replyToMessageID int) error {
	session, err := loadRecommendationSession(ctx, userID)
	if err != nil {
		session = &recommendationSession{}
	}
	artwork, score, err := pickScoredRecommendation(ctx, serv, userID, session)
	if err != nil {
		return oops.Wrapf(err, "failed to pick recommendation artwork")
	}
	if artwork == nil {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "暂时没有可推荐的作品，请先在私聊中点赞/点踩一些作品建立偏好").WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	session.CurrentSourceURL = artwork.GetSourceURL()
	session.CurrentTags = artwork.GetTags()
	if err := saveRecommendationSession(ctx, userID, session); err != nil {
		return oops.Wrapf(err, "failed to save recommendation session")
	}

	picture := artwork.FirstMedia()
	if picture == nil {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "这篇作品暂时没有图片可展示").WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	picLike, ok := picture.(shared.PictureLike)
	if !ok {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "这篇作品暂时没有图片可展示").WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	file, err := utils.GetPicturePhotoInputFile(ctx, serv, meta, picLike)
	if err != nil {
		return oops.Wrapf(err, "failed to get photo input file")
	}
	defer file.Close()
	caption := fmt.Sprintf("%s\n\n匹配度: %.0f%% · 已收藏 %d 个作品", utils.ArtworkHTMLCaption(artwork), score*100, len(session.LikedSourceURLs))
	photo := telegoutil.Photo(chatID, file.Value).
		WithCaption(caption).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telegoutil.InlineKeyboardButton("👍 喜欢").WithCallbackData("recommend_like"),
				telegoutil.InlineKeyboardButton("👎 不喜欢").WithCallbackData("recommend_dislike"),
			),
			telegoutil.InlineKeyboardRow(
				telegoutil.InlineKeyboardButton("⏭️ 下一个").WithCallbackData("recommend_next"),
				telegoutil.InlineKeyboardButton("📤 推送已喜欢").WithCallbackData("recommend_push"),
			),
		))
	if replyToMessageID != 0 {
		photo = photo.WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID})
	}
	if artwork.GetR18() {
		photo = photo.WithHasSpoiler()
	}
	_, err = tgCtx.Bot().SendPhoto(ctx, photo)
	return err
}

// pickScoredRecommendation 基于用户 XP 画像(偏好权重)推荐作品:
//  1. 取用户偏好权重最高的 tag
//  2. 通过 AI API 扩展为更多关联/相似 tag(若启用)
//  3. 用这些 tag 在 Pixiv 搜索全新作品
//  4. 按 XP 画像匹配分数从高到低排序, 返回得分最高且未发布/未看过的作品及其分数
//  5. 若 Pixiv 搜索失败或无结果, 回退到从数据库已有作品按 XP 画像直接推荐
func pickScoredRecommendation(ctx context.Context, serv *service.Service, userID int64, session *recommendationSession) (shared.ArtworkLike, float64, error) {
	// 1. 获取用户 XP 画像
	pref, err := service.GetUserPreference(ctx, userID)
	if err != nil {
		pref = &service.UserPreference{
			PositiveWeights: make(map[string]float64),
			NegativeWeights: make(map[string]float64),
		}
	}

	// 2. 取权重最高的 tag (无画像时用群/频道近期画像兜底)
	topTags := service.TopPreferenceTags(pref, 8)
	if len(topTags) == 0 {
		profile := serv.BuildRecentTagProfile(ctx, 10000)
		topTags = service.TopProfileTags(profile, 8)
	}

	// 3. 排除已看过/已喜欢的
	seen := make(map[string]struct{}, len(session.SeenSourceURLs)+len(session.LikedSourceURLs))
	for _, u := range session.SeenSourceURLs {
		seen[u] = struct{}{}
	}
	for _, u := range session.LikedSourceURLs {
		seen[u] = struct{}{}
	}

	// 4. 若没有可用 tag 画像, 直接从数据库按 XP 画像推荐已有作品
	if len(topTags) == 0 {
		return pickFromDBByXP(ctx, serv, userID, pref, seen)
	}
	tagNames := make([]string, 0, len(topTags))
	for _, tw := range topTags {
		tagNames = append(tagNames, tw.Tag)
	}

	// 5. AI 扩展关联 tag (失败时回退到原始 tags), 控制总数避免搜索词过长
	searchTags := tagNames
	expanded, expandErr := serv.ExpandTagsWithAI(ctx, tagNames)
	if expandErr == nil && len(expanded) > 0 {
		searchTags = append([]string{}, tagNames...)
		searchTags = append(searchTags, expanded...)
		if len(searchTags) > 12 {
			searchTags = searchTags[:12]
		}
	}

	// 6. 按 tag 搜索全新作品
	fetched, err := serv.SearchNewArtworksByTags(ctx, searchTags, 50)
	if err != nil {
		log.Warn("recommend: tag search failed, fallback to rss fetch", "err", err)
		fetched, err = serv.FetchNewArtworks(ctx, 50)
	}
	if err != nil {
		log.Warn("recommend: fetch failed, fallback to db recommend", "err", err)
		return pickFromDBByXP(ctx, serv, userID, pref, seen)
	}
	if len(fetched) == 0 {
		log.Warn("recommend: no new artworks fetched, fallback to db recommend")
		return pickFromDBByXP(ctx, serv, userID, pref, seen)
	}

	// 7. 按 XP 画像分数从高到低选出最佳
	best, bestURL, bestScore, err := serv.PickBestRecommendationByXP(ctx, fetched, pref, seen)
	if err != nil {
		return nil, 0, oops.Wrapf(err, "failed to pick best recommendation by xp profile")
	}
	if best == nil {
		log.Warn("recommend: no scored new artwork matched, fallback to db recommend")
		return pickFromDBByXP(ctx, serv, userID, pref, seen)
	}

	// 8. 对选中的作品获取完整详情(含原图), 搜索结果条目只有封面缩略图
	full, err := serv.FetchArtworkInfo(ctx, bestURL)
	if err != nil {
		log.Warn("recommend: failed to fetch full artwork info, fallback to search item", "url", bestURL, "err", err)
		full = best
	}

	cached, err := service.ConvertFetchedToCached(full)
	if err != nil {
		return nil, 0, oops.Wrapf(err, "failed to convert fetched artwork")
	}

	session.addSeen(bestURL)
	_ = saveRecommendationSession(ctx, userID, session)
	return cached, bestScore, nil
}

// pickFromDBByXP 从数据库已有作品中按用户 XP 画像推荐得分最高的一个。
// 用户历史记录(已发布/已入库作品)的 tag 即为其画像来源之一。
func pickFromDBByXP(ctx context.Context, serv *service.Service, userID int64, pref *service.UserPreference, seen map[string]struct{}) (shared.ArtworkLike, float64, error) {
	seenURLs := make([]string, 0, len(seen))
	for u := range seen {
		seenURLs = append(seenURLs, u)
	}
	artwork, err := serv.FetchAndScoreRecommendations(ctx, userID, 100, seenURLs)
	if err != nil {
		return nil, 0, oops.Wrapf(err, "failed to fetch and score db recommendations")
	}
	if artwork == nil {
		return nil, 0, nil
	}
	score := service.CalculateMatchScore(artwork.GetTags(), pref)
	return artwork, score, nil
}

func pushRecommendationSelection(ctx context.Context, tgCtx *telegohandler.Context, serv *service.Service, meta *metautil.MetaData, chatID telego.ChatID, messageID int, sourceURLs []string) (int, error) {
	if meta.ChannelAvailable() == false {
		return 0, oops.New("频道未配置")
	}
	count := 0
	for _, sourceURL := range sourceURLs {
		// Check if artwork already exists in DB
		awEnt, err := serv.GetArtworkByURL(ctx, sourceURL)
		if err == nil && awEnt != nil {
			// Artwork already created - just send to channel
			targetChatID := meta.ResolvePostChatID(awEnt)
			results, err := utils.SendArtworkMediaGroup(ctx, tgCtx.Bot(), serv, meta, targetChatID, awEnt)
			if err != nil {
				log.Warn("failed to send created artwork to channel", "url", sourceURL, "err", err)
				continue
			}
			if len(results) > 0 {
				count++
			}
			continue
		}
		// Artwork not yet created - fetch cached and create
		cachedArtwork, err := serv.GetOrFetchCachedArtwork(ctx, sourceURL)
		if err != nil {
			continue
		}
		artwork := cachedArtwork.Artwork.Data()
		if err := utils.PostAndCreateArtwork(ctx, tgCtx.Bot(), serv, meta, artwork, chatID, meta.ChannelChatID(), messageID); err != nil {
			continue
		}
		count++
	}
	return count, nil
}
