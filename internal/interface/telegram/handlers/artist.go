package handlers

import (
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
)

// artistLinkBatchSize 每批发送的链接数量。
// 每行链接约 40 字符, 90 行约 3600 字符, 留余量避免超过 Telegram 单条消息 4096 字符限制。
const artistLinkBatchSize = 90

// handleArtistPageURL 输出画师主页下所有作品的完整链接, 不做任何说明。
// 链接过多时按消息长度限制分批发送; 拉取失败时静默忽略。
func handleArtistPageURL(ctx *telegohandler.Context, m HandlerManager, message telego.Message, artistURL string) error {
	urls, err := m.Service.FetchArtistArtworks(ctx, artistURL, 0)
	if err != nil {
		// 拉取失败(如未配置 refresh_token / API 错误 / 非画师链接)时静默忽略
		return ctx.Err()
	}
	if len(urls) == 0 {
		return ctx.Err()
	}
	for start := 0; start < len(urls); start += artistLinkBatchSize {
		end := start + artistLinkBatchSize
		if end > len(urls) {
			end = len(urls)
		}
		utils.ReplyMessage(ctx, message, strings.Join(urls[start:end], "\n"))
	}
	return ctx.Err()
}
