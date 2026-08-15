package telegram

import (
	"context"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/samber/oops"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/model/entity"
	"github.com/wwwangzilin/LotsACG/internal/shared"
)

func (b *BotApp) PostAndCreateArtwork(ctx context.Context, artwork *entity.CachedArtworkData) error {
	var adminId telego.ChatID
	if len(b.cfg.Admins) > 0 {
		adminId = telegoutil.ID(b.cfg.Admins[0])
	} else {
		adminIds, _ := b.serv.GetAdminUserIDs(ctx)
		if len(adminIds) > 0 {
			adminId = telegoutil.ID(adminIds[0])
		}
	}
	if err := utils.PostAndCreateArtwork(ctx, b.Bot(), b.serv, b.meta, artwork, adminId, b.meta.ChannelChatID(), 0); err != nil {
		return oops.Wrapf(err, "posting and creating artwork %s", artwork.SourceURL)
	}
	return nil
}

type artworkInfoTask struct {
	ctx           context.Context
	sourceUrl     string
	chatID        int64
	appendCaption string
}

func (b *BotApp) SendArtworkInfo(ctx context.Context, sourceUrl string, chatID int64, appendCaption string) {
	b.artworkInfoQueue <- artworkInfoTask{
		ctx:           ctx,
		sourceUrl:     sourceUrl,
		chatID:        chatID,
		appendCaption: appendCaption,
	}
}

// PostArtworkToChannel 将指定来源链接的作品发布到主频道 (供 XP-Pusher 等外部调用)。
func (b *BotApp) PostArtworkToChannel(ctx context.Context, sourceURL string) error {
	cachedArtwork, err := b.serv.GetOrFetchCachedArtwork(ctx, sourceURL)
	if err != nil {
		return oops.Wrapf(err, "failed to get or fetch cached artwork")
	}
	if cachedArtwork.Status != shared.ArtworkStatusCached {
		return oops.New("artwork already posted or being posted")
	}
	artwork := cachedArtwork.Artwork.Data()
	if artwork == nil || len(artwork.Pictures) == 0 {
		return oops.New("artwork has no pictures")
	}
	if err := utils.PostAndCreateArtwork(ctx, b.Bot(), b.serv, b.meta, artwork, telego.ChatID{}, b.meta.ChannelChatID(), 0); err != nil {
		return oops.Wrapf(err, "failed to post artwork to channel")
	}
	return nil
}

// SendArtworkNotification 实现 scheduler.ArtworkNotifier: 向用户推送新作品 (画师关注/标签订阅)。
func (b *BotApp) SendArtworkNotification(ctx context.Context, userID int64, sourceURL string) error {
	b.SendArtworkInfo(ctx, sourceURL, userID, "")
	return nil
}

// SendTextToUser 向指定用户发送一条文本消息。
func (b *BotApp) SendTextToUser(ctx context.Context, userID int64, text string) error {
	_, err := b.Bot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    telegoutil.ID(userID),
		Text:      text,
		ParseMode: telego.ModeHTML,
	})
	return err
}
