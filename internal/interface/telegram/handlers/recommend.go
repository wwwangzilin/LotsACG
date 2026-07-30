package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/krau/LotsACG/internal/infra/kvstor"
	"github.com/krau/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/krau/LotsACG/internal/interface/telegram/metautil"
	"github.com/krau/LotsACG/internal/model/query"
	"github.com/krau/LotsACG/internal/service"
	"github.com/krau/LotsACG/internal/shared"
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
		if session.CurrentSourceURL != "" {
			session.addLike(session.CurrentSourceURL)
			_ = saveRecommendationSession(ctx, query.From.ID, session)
		}
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "已加入收藏", CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_next":
		ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "为你换一个推荐", CacheTime: 10})
		return sendRecommendation(ctx, ctx, query.Message.GetChat().ChatID(), query.From.ID, serv, meta, query.Message.GetMessageID())
	case "recommend_push":
		if len(session.LikedSourceURLs) == 0 {
			ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID, Text: "还没有收藏任何作品", ShowAlert: true, CacheTime: 30})
			return nil
		}
		count, err := pushRecommendationSelection(ctx, ctx, serv, meta, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), session.LikedSourceURLs)
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

func sendRecommendation(ctx context.Context, tgCtx *telegohandler.Context, chatID telego.ChatID, userID int64, serv *service.Service, meta *metautil.MetaData, replyToMessageID int) error {
	session, err := loadRecommendationSession(ctx, userID)
	if err != nil {
		session = &recommendationSession{}
	}
	artwork, err := pickRecommendationArtwork(ctx, serv, session)
	if err != nil {
		return oops.Wrapf(err, "failed to pick recommendation artwork")
	}
	if artwork == nil {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "暂时没有可推荐的作品").WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	session.CurrentSourceURL = artwork.SourceURL
	if err := saveRecommendationSession(ctx, userID, session); err != nil {
		return oops.Wrapf(err, "failed to save recommendation session")
	}
	if len(artwork.Pictures) == 0 {
		_, err := tgCtx.Bot().SendMessage(ctx, telegoutil.Message(chatID, "这篇作品暂时没有图片可展示").WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID}))
		return err
	}
	picture := artwork.Pictures[0]
	file, err := utils.GetPicturePhotoInputFile(ctx, serv, meta, picture)
	if err != nil {
		return oops.Wrapf(err, "failed to get photo input file")
	}
	defer file.Close()
	caption := fmt.Sprintf("%s\n\n已收藏 %d 个作品", utils.ArtworkHTMLCaption(artwork), len(session.LikedSourceURLs))
	photo := telegoutil.Photo(chatID, file.Value).
		WithCaption(caption).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telegoutil.InlineKeyboardButton("👍 喜欢").WithCallbackData("recommend_like"),
				telegoutil.InlineKeyboardButton("⏭️ 下一个").WithCallbackData("recommend_next"),
			),
			telegoutil.InlineKeyboardRow(
				telegoutil.InlineKeyboardButton("📤 推送已喜欢").WithCallbackData("recommend_push"),
			),
		))
	if replyToMessageID != 0 {
		photo = photo.WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMessageID})
	}
	if artwork.R18 {
		photo = photo.WithHasSpoiler()
	}
	_, err = tgCtx.Bot().SendPhoto(ctx, photo)
	return err
}

func pickRecommendationArtwork(ctx context.Context, serv *service.Service, session *recommendationSession) (shared.ArtworkLike, error) {
	// Keep the implementation simple and resilient by trying a few random picks.
	for i := 0; i < 5; i++ {
		artworks, err := serv.QueryArtworks(ctx, query.ArtworksDB{
			ArtworksFilter: query.ArtworksFilter{
				HasPicture: true,
			},
			Paginate: query.Paginate{Offset: 0, Limit: 1},
			Random:   true,
		})
		if err != nil {
			return nil, err
		}
		if len(artworks) == 0 {
			continue
		}
		artwork := artworks[0]
		if isSeenSourceURL(session, artwork.SourceURL) {
			continue
		}
		if session != nil {
			session.SeenSourceURLs = append(session.SeenSourceURLs, artwork.SourceURL)
			if len(session.SeenSourceURLs) > 10 {
				session.SeenSourceURLs = session.SeenSourceURLs[len(session.SeenSourceURLs)-10:]
			}
		}
		return artwork, nil
	}
	return nil, nil
}

func isSeenSourceURL(session *recommendationSession, sourceURL string) bool {
	if session == nil || sourceURL == "" {
		return false
	}
	for _, seen := range session.SeenSourceURLs {
		if seen == sourceURL {
			return true
		}
	}
	return false
}

func pushRecommendationSelection(ctx context.Context, tgCtx *telegohandler.Context, serv *service.Service, meta *metautil.MetaData, chatID telego.ChatID, messageID int, sourceURLs []string) (int, error) {
	if meta.ChannelAvailable() == false {
		return 0, oops.New("频道未配置")
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
