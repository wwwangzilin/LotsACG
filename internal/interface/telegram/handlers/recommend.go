package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/metautil"
	"github.com/wwwangzilin/LotsACG/internal/model/entity"
	"github.com/wwwangzilin/LotsACG/internal/model/query"
	"github.com/wwwangzilin/LotsACG/internal/service"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/samber/oops"
)

type recommendationSession struct {
	LikedSourceURLs  []string `json:"liked_source_urls"`
	SeenSourceURLs   []string `json:"seen_source_urls"`
	CurrentSourceURL string   `json:"current_source_url"`
}

func Recommend(ctx *telegohandler.Context, message telego.Message) error {
	if message.Chat.Type != telego.ChatTypePrivate {
		utils.ReplyMessage(ctx, message, "杩欎釜鍔熻兘浠呮敮鎸佸湪绉佽亰涓娇鐢?)
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
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "璇峰湪绉佽亰涓娇鐢?, ShowAlert: true, CacheTime: 60})
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
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "宸插姞鍏ユ敹钘忥紝宸茶浣忎綘鐨勫亸濂?, CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_dislike":
		_ = updatePreferenceFromSession(ctx, serv, query.From.ID, session, false)
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "宸茶褰曪紝浼氬噺灏戣繖绫绘帹鑽?, CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_next":
		if session.CurrentSourceURL != "" {
			session.addSeen(session.CurrentSourceURL)
			_ = saveRecommendationSession(ctx, query.From.ID, session)
		}
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "涓轰綘鎹竴涓帹鑽?, CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_push":
		if len(session.LikedSourceURLs) == 0 {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "杩樻病鏈夋敹钘忎换浣曚綔鍝?, ShowAlert: true, CacheTime: 30})
			return nil
		}
		count, err := pushRecommendationSelection(ctx, ctx, serv, meta, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), session.LikedSourceURLs)
		if err != nil {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "鎺ㄩ€佸け璐? " + err.Error(), ShowAlert: true, CacheTime: 30})
			return nil
		}
		session.LikedSourceURLs = nil
		_ = saveRecommendationSession(ctx, query.From.ID, session)
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: fmt.Sprintf("宸叉帹閫?%d 涓綔鍝佸埌棰戦亾", count), CacheTime: 10})
		return nil
	default:
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "鏃犳晥鎿嶄綔", CacheTime: 10})
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
	awEnt, err := serv.GetArtworkByURL(ctx, session.CurrentSourceURL)
	if err != nil {
		// Artwork may not be persisted yet (cached only). Try cached.
		return nil
	}
	tags := extractTagNames(awEnt.Tags)
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
	artwork, err := pickScoredRecommendation(ctx, serv, userID, session)
	if err != nil {
		return oops.Wrapf(err, "failed to pick recommendation artwork")
	}
	if artwork == nil {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "鏆傛椂娌℃湁鍙帹鑽愮殑浣滃搧").WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	session.CurrentSourceURL = artwork.GetSourceURL()
	if err := saveRecommendationSession(ctx, userID, session); err != nil {
		return oops.Wrapf(err, "failed to save recommendation session")
	}

	awEntity, ok := artwork.(*entity.Artwork)
	if !ok || len(awEntity.Pictures) == 0 {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "杩欑瘒浣滃搧鏆傛椂娌℃湁鍥剧墖鍙睍绀?).WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	picture := awEntity.Pictures[0]
	file, err := utils.GetPicturePhotoInputFile(ctx, serv, meta, picture)
	if err != nil {
		return oops.Wrapf(err, "failed to get photo input file")
	}
	defer file.Close()
	caption := fmt.Sprintf("%s\n\n宸叉敹钘?%d 涓綔鍝?, utils.ArtworkHTMLCaption(artwork), len(session.LikedSourceURLs))
	photo := telegoutil.Photo(chatID, file.Value).
		WithCaption(caption).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telegoutil.InlineKeyboardButton("馃憤 鍠滄").WithCallbackData("recommend_like"),
				telegoutil.InlineKeyboardButton("馃憥 涓嶅枩娆?).WithCallbackData("recommend_dislike"),
			),
			telegoutil.InlineKeyboardRow(
				telegoutil.InlineKeyboardButton("鈴笍 涓嬩竴涓?).WithCallbackData("recommend_next"),
				telegoutil.InlineKeyboardButton("馃摛 鎺ㄩ€佸凡鍠滄").WithCallbackData("recommend_push"),
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

// pickScoredRecommendation fetches a batch of random artworks and scores them
// against the user's preference profile, returning the highest-scoring unseen one.
func pickScoredRecommendation(ctx context.Context, serv *service.Service, userID int64, session *recommendationSession) (shared.ArtworkLike, error) {
	pref, err := service.GetUserPreference(ctx, userID)
	if err != nil {
		return nil, oops.Wrapf(err, "failed to get user preference")
	}
	// Determine batch size: if user has preferences, fetch more candidates for better scoring.
	batchSize := 20
	if len(pref.PositiveWeights) == 0 && len(pref.NegativeWeights) == 0 {
		// Cold start: no preference data yet, just try random picks
		batchSize = 5
	}

	aw, err := serv.QueryArtworks(ctx, query.ArtworksDB{
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

	seenURLs := session.SeenSourceURLs
	seen := make(map[string]struct{}, len(seenURLs)+len(session.LikedSourceURLs))
	for _, u := range seenURLs {
		seen[u] = struct{}{}
	}
	for _, u := range session.LikedSourceURLs {
		seen[u] = struct{}{}
	}

	artworkLikes := make([]shared.ArtworkLike, 0, len(aw))
	for _, a := range aw {
		if _, ok := seen[a.SourceURL]; ok {
			continue
		}
		artworkLikes = append(artworkLikes, a)
	}
	if len(artworkLikes) == 0 {
		return nil, nil
	}

	best := service.PickBestRecommendation(artworkLikes, nil, pref)
	if best == nil && len(artworkLikes) > 0 {
		best = artworkLikes[0]
	}

	if best != nil {
		session.addSeen(best.GetSourceURL())
		_ = saveRecommendationSession(ctx, userID, session)
	}
	return best, nil
}

func pushRecommendationSelection(ctx context.Context, tgCtx *telegohandler.Context, serv *service.Service, meta *metautil.MetaData, chatID telego.ChatID, messageID int, sourceURLs []string) (int, error) {
	if meta.ChannelAvailable() == false {
		return 0, oops.New("棰戦亾鏈厤缃?)
	}
	count := 0
	for _, sourceURL := range sourceURLs {
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