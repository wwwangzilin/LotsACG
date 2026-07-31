package service

import (
	"sort"
	"strings"

	"github.com/samber/oops"
	"github.com/wwwangzilin/LotsACG/internal/model/dto"
	"github.com/wwwangzilin/LotsACG/internal/model/entity"
)

// normalizeTag 归一化 tag: 小写 + 空格转下划线。
func normalizeTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, " ", "_")
	return tag
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

// TopProfileTags 从画像中取出权重最高的 n 个 tag, 按权重从高到低排序。
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
