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
	"github.com/wwwangzilin/LotsACG/internal/model/dto"
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
	// 当前推荐作品的完整数据, 用于推送到群时避免重新 fetch 失败
	CurrentArtworkData *entity.CachedArtworkData `json:"current_artwork_data,omitempty"`
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
	case "recommend_push", "recommend_push_current":
		if session.CurrentSourceURL == "" {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "当前没有作品可推送", ShowAlert: true, CacheTime: 30})
			return nil
		}
		count, err := pushRecommendationSelection(ctx, ctx, serv, meta, query.Message.GetChat().ChatID(), 0, []string{session.CurrentSourceURL}, session)
		if err != nil {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "推送失败: " + err.Error(), ShowAlert: true, CacheTime: 30})
			return nil
		}
		if count == 0 {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "推送失败，请查看日志", ShowAlert: true, CacheTime: 30})
			return nil
		}
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "已推送到群", CacheTime: 10})
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
	if cd, ok := artwork.(*entity.CachedArtworkData); ok {
		session.CurrentArtworkData = cd
	}
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
	caption := fmt.Sprintf("%s\n\n匹配度: %.0f%%", utils.ArtworkHTMLCaption(artwork), score*100)
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
				telegoutil.InlineKeyboardButton("📤 推送到群").WithCallbackData("recommend_push_current"),
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

// pickScoredRecommendation 基于用户 XP 画像推荐 Pixiv 全新作品 (移植 XP-Pusher):
//  1. 构建用户 XP 画像 (偏好 + 群历史, TF-IDF + 时间衰减)
//  2. 组合搜索 + 单 tag 兜底 (XPDiscover)
//  3. 过滤排序: 匹配度 + 收藏数 + AI 精排 (XPFilterAndRank)
//  4. 排除已发布/已看过, 返回得分最高的新作品
//     注意: 推荐始终来自 Pixiv 新图, 绝不会回退到数据库中已发布的作品。
func pickScoredRecommendation(ctx context.Context, serv *service.Service, userID int64, session *recommendationSession) (shared.ArtworkLike, float64, error) {
	// 1. 获取用户偏好 (XP 画像基础)
	pref, err := service.GetUserPreference(ctx, userID)
	if err != nil {
		pref = &service.UserPreference{
			PositiveWeights: make(map[string]float64),
			NegativeWeights: make(map[string]float64),
			TagUpdatedAt:    make(map[string]time.Time),
			TagPairs:        make(map[string]float64),
		}
	}

	// 2. 构建完整 XP 画像 (偏好 + 群历史)
	profile, err := serv.BuildUserXPProfile(ctx, userID)
	if err != nil {
		log.Warn("recommend: failed to build user xp profile", "err", err)
	}

	// 3. 排除已看过/已喜欢的
	seen := make(map[string]struct{}, len(session.SeenSourceURLs)+len(session.LikedSourceURLs))
	for _, u := range session.SeenSourceURLs {
		seen[u] = struct{}{}
	}
	for _, u := range session.LikedSourceURLs {
		seen[u] = struct{}{}
	}

	// 4. 组合搜索 + 单 tag 兜底, 拉取全新作品 (应用用户 R18 模式与图源选择)
	r18Mode, _ := service.GetUserR18Mode(ctx, userID)
	pixivMode := r18Mode.ToPixivMode()
	recSource := service.GetUserRecommendSource(ctx, userID)
	fetched, err := serv.XPDiscover(ctx, pref, profile, 50, pixivMode, recSource)
	if err != nil {
		return nil, 0, oops.Wrapf(err, "failed to discover new artworks")
	}
	if len(fetched) == 0 {
		return nil, 0, nil
	}

	// 4.5 候选层强过滤 (防御: 搜索/RSS 可能漏网, 保证候选符合用户 R18 模式)
	fetched = filterFetchedByR18Mode(fetched, pixivMode)
	if len(fetched) == 0 {
		return nil, 0, nil
	}

	// 5. 过滤排序 (匹配度 + 收藏数 + AI 精排)
	ranked, err := serv.XPFilterAndRank(ctx, fetched, profile, pref, seen)
	if err != nil {
		return nil, 0, oops.Wrapf(err, "failed to filter and rank artworks")
	}
	if len(ranked) == 0 {
		return nil, 0, nil
	}

	// 6. 按用户 R18 模式筛选并获取完整详情(含原图)。
	//    注意: 搜索结果条目用 xRestrict 判 R18, 详情接口用 tag 判 R18,
	//    两者可能不一致, 因此以详情结果为准, 不符合模式则跳过该候选。
	var best *dto.FetchedArtwork
	var bestScore float64
	for _, candidate := range ranked {
		art := candidate.Artwork
		if !artMatchesR18Mode(art.R18, pixivMode) {
			continue
		}
		full, err := serv.FetchArtworkInfo(ctx, art.SourceURL)
		if err != nil {
			log.Warn("recommend: failed to fetch full artwork info, fallback to search item", "url", art.SourceURL, "err", err)
			full = art
		}
		if !artMatchesR18Mode(full.R18, pixivMode) {
			continue
		}
		best = full
		bestScore = candidate.Score
		break
	}
	if best == nil {
		return nil, 0, nil
	}
	bestURL := best.SourceURL

	cached, err := service.ConvertFetchedToCached(best)
	if err != nil {
		return nil, 0, oops.Wrapf(err, "failed to convert fetched artwork")
	}

	session.addSeen(bestURL)
	_ = saveRecommendationSession(ctx, userID, session)
	return cached, bestScore, nil
}

// filterFetchedByR18Mode 按 R18 模式过滤候选列表。
// pixivMode: all(全部) / safe(全年龄) / r18(仅 R18)。
func filterFetchedByR18Mode(fetched []*dto.FetchedArtwork, pixivMode string) []*dto.FetchedArtwork {
	filtered := make([]*dto.FetchedArtwork, 0, len(fetched))
	for _, art := range fetched {
		if art == nil {
			continue
		}
		if artMatchesR18Mode(art.R18, pixivMode) {
			filtered = append(filtered, art)
		}
	}
	return filtered
}

// artMatchesR18Mode 判断作品的 R18 状态是否符合指定模式。
func artMatchesR18Mode(isR18 bool, pixivMode string) bool {
	switch pixivMode {
	case "safe":
		return !isR18
	case "r18":
		return isR18
	default: // all
		return true
	}
}

func pushRecommendationSelection(ctx context.Context, tgCtx *telegohandler.Context, serv *service.Service, meta *metautil.MetaData, chatID telego.ChatID, messageID int, sourceURLs []string, session *recommendationSession) (int, error) {
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

		// 优先使用会话中已缓存的完整作品数据 (避免重新 fetch 失败导致没有图片)
		var artwork *entity.CachedArtworkData
		if session != nil && session.CurrentArtworkData != nil && session.CurrentArtworkData.SourceURL == sourceURL {
			artwork = session.CurrentArtworkData
		} else {
			cachedArtwork, err := serv.GetOrFetchCachedArtwork(ctx, sourceURL)
			if err != nil {
				log.Warn("failed to get or fetch cached artwork for push", "url", sourceURL, "err", err)
				continue
			}
			artwork = cachedArtwork.Artwork.Data()
		}
		if artwork == nil || len(artwork.Pictures) == 0 {
			log.Warn("recommend push: artwork has no pictures", "url", sourceURL)
			continue
		}
		if err := utils.PostAndCreateArtwork(ctx, tgCtx.Bot(), serv, meta, artwork, chatID, meta.ChannelChatID(), messageID); err != nil {
			log.Warn("failed to post and create artwork from recommend", "url", sourceURL, "err", err)
			continue
		}
		count++
	}
	return count, nil
}
