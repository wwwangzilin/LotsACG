package service

import (
	"context"
	"fmt"
	"time"

	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
)

// recommendSourceTTL 用户推荐图源选择的过期时间。
const recommendSourceTTL = 30 * 24 * time.Hour

func recommendSourceKey(userID int64) string {
	return fmt.Sprintf("recsource:%d", userID)
}

// GetUserRecommendSource 返回用户选择的推荐图源 (空字符串 = 全部源)。
func GetUserRecommendSource(ctx context.Context, userID int64) string {
	src, err := kvstor.Get[string](ctx, recommendSourceKey(userID))
	if err != nil {
		return ""
	}
	return src
}

// SetUserRecommendSource 设置用户推荐图源 (空 = 全部源)。
func SetUserRecommendSource(ctx context.Context, userID int64, sourceType string) error {
	return kvstor.SetWithTTL(ctx, recommendSourceKey(userID), sourceType, recommendSourceTTL)
}

// recommendMinBookmarksTTL 用户推荐最小收藏数设置的过期时间。
const recommendMinBookmarksTTL = 30 * 24 * time.Hour

func recommendMinBookmarksKey(userID int64) string {
	return fmt.Sprintf("recminbm:%d", userID)
}

// GetUserMinBookmarks 返回用户设置的推荐最小收藏数 (0 = 不限制)。
func GetUserMinBookmarks(ctx context.Context, userID int64) int {
	v, err := kvstor.Get[int](ctx, recommendMinBookmarksKey(userID))
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// SetUserMinBookmarks 设置用户推荐最小收藏数 (0 = 不限制)。
func SetUserMinBookmarks(ctx context.Context, userID int64, min int) error {
	if min < 0 {
		min = 0
	}
	return kvstor.SetWithTTL(ctx, recommendMinBookmarksKey(userID), min, recommendMinBookmarksTTL)
}
