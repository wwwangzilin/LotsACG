package handlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/service"
)

// XpProfile 查看当前用户的 XP 画像(偏好标签权重)。
// 用法: /xp
func XpProfile(ctx *telegohandler.Context, message telego.Message) error {
	if message.Chat.Type != telego.ChatTypePrivate {
		utils.ReplyMessage(ctx, message, "这个功能仅支持在私聊中使用")
		return nil
	}
	serv, err := requireService(ctx)
	if err != nil {
		return err
	}
	_ = serv
	pref, err := service.GetUserPreference(ctx, message.From.ID)
	if err != nil {
		pref = &service.UserPreference{}
	}

	if len(pref.PositiveWeights) == 0 && len(pref.NegativeWeights) == 0 {
		utils.ReplyMessage(ctx, message, "你还没有任何偏好记录。\n在私聊中使用 /recommend 推荐作品，点击 👍 或 👎 即可建立你的 XP 画像。")
		return nil
	}

	formatWeights := func(weights map[string]float64) string {
		type kv struct {
			tag    string
			weight float64
		}
		items := make([]kv, 0, len(weights))
		for t, w := range weights {
			if w <= 0 {
				continue
			}
			items = append(items, kv{tag: t, weight: w})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].weight > items[j].weight })
		var sb strings.Builder
		for i, item := range items {
			if i >= 30 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %s (%.1f)\n", item.tag, item.weight))
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	text := "🎨 <b>你的 XP 画像</b>\n"
	text += "\n<b>👍 喜欢的标签:</b>\n"
	if pos := formatWeights(pref.PositiveWeights); pos != "" {
		text += pos
	} else {
		text += "  (无)"
	}
	text += "\n\n<b>👎 不喜欢的标签:</b>\n"
	if neg := formatWeights(pref.NegativeWeights); neg != "" {
		text += neg
	} else {
		text += "  (无)"
	}
	text += "\n\n<b>🔗 常用组合:</b>\n"
	if pairs := formatPairs(pref.TagPairs); pairs != "" {
		text += pairs
	} else {
		text += "  (无)"
	}
	text += "\n\n使用 /recommend 获取推荐, 点击 👍/👎 会实时更新画像"
	utils.ReplyMessageWithHTML(ctx, message, text)
	return nil
}

// formatPairs 格式化 tag 组合 (权重最高的前 10 个)。
func formatPairs(pairs map[string]float64) string {
	if len(pairs) == 0 {
		return ""
	}
	type kv struct {
		pair   string
		weight float64
	}
	items := make([]kv, 0, len(pairs))
	for p, w := range pairs {
		if w <= 0 {
			continue
		}
		items = append(items, kv{pair: strings.ReplaceAll(p, "|", " + "), weight: w})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].weight > items[j].weight })
	var sb strings.Builder
	for i, item := range items {
		if i >= 10 {
			break
		}
		sb.WriteString(fmt.Sprintf("  %s\n", item.pair))
	}
	return strings.TrimRight(sb.String(), "\n")
}
