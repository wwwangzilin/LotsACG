package pixiv

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/goccy/go-json"
	"github.com/wwwangzilin/LotsACG/internal/model/dto"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/wwwangzilin/LotsACG/pkg/log"
	"github.com/samber/oops"
)

// SearchArtworksByTags 通过 Pixiv 搜索接口按 tag 搜索最新作品。
// 搜索多个 tag 会按空格组合 (OR 匹配)。返回轻量条目(含 title/tags/封面),
// 便于快速打分; 完整图片信息可对选中的作品再调用 GetArtworkInfo。
func (p *Pixiv) SearchArtworksByTags(ctx context.Context, tags []string, limit int) ([]*dto.FetchedArtwork, error) {
	if len(tags) == 0 {
		return nil, oops.New("no tags provided for pixiv search")
	}
	if limit <= 0 {
		limit = 20
	}
	// Pixiv 搜索关键词: 多个 tag 用空格分隔 (OR 语义)
	keyword := strings.Join(tags, " ")
	searchURL := "https://www.pixiv.net/ajax/search/artworks/" + url.PathEscape(keyword) +
		"?word=" + url.QueryEscape(keyword) +
		"&order=date_d&mode=all&p=1&s_mode=s_tag&type=all&lang=zh"

	var lastErr error
	for i := 0; i < len(p.reqClients); i++ {
		client := p.nextClient()
		resp, err := client.R().SetContext(ctx).Get(searchURL)
		if err != nil {
			lastErr = err
			log.Warnf("pixiv tag search request failed with account %d: %v", i+1, err)
			continue
		}
		var searchResp PixivSearchResp
		if err := json.Unmarshal(resp.Bytes(), &searchResp); err != nil {
			lastErr = oops.Wrapf(err, "unmarshal pixiv search response")
			log.Warnf("pixiv tag search unmarshal failed with account %d: %v", i+1, err)
			continue
		}
		if searchResp.Error {
			lastErr = oops.Errorf("pixiv search response error: %s", searchResp.Message)
			log.Warnf("pixiv tag search returned error with account %d: %s", i+1, searchResp.Message)
			continue
		}
		if searchResp.Body == nil || searchResp.Body.IllustManga == nil || len(searchResp.Body.IllustManga.Data) == 0 {
			return nil, nil
		}
		artworks := make([]*dto.FetchedArtwork, 0, limit)
		for _, item := range searchResp.Body.IllustManga.Data {
			if len(artworks) >= limit {
				break
			}
			// 只取普通插画(illustType 0), 跳过动图
			if item.IllustType != 0 {
				continue
			}
			fetched := searchIllustToFetched(item, p.cfg.ImgProxy)
			if fetched == nil {
				continue
			}
			artworks = append(artworks, fetched)
		}
		if len(artworks) > 0 {
			return artworks, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, oops.New("no pixiv accounts available")
}

// searchIllustToFetched 将搜索结果中的单个插画条目转换为轻量 FetchedArtwork。
// 仅包含 title/tags/封面缩略图, 用于快速打分排序。
func searchIllustToFetched(item *PixivSearchIllustData, imgProxy string) *dto.FetchedArtwork {
	if item == nil || item.ID == "" || item.URL == "" {
		return nil
	}
	original := strings.Replace(item.URL, "i.pximg.net", imgProxy, 1)
	tags := make([]string, 0, len(item.Tags))
	for _, t := range item.Tags {
		if t.Tag != "" {
			tags = append(tags, t.Tag)
		}
	}
	fetched := &dto.FetchedArtwork{
		Title:       item.Title,
		Description: item.Description,
		R18:         item.XRestrict != 0,
		SourceType:  shared.SourceTypePixiv,
		SourceURL:   fmt.Sprintf("https://www.pixiv.net/artworks/%s", item.ID),
		Tags:        tags,
		Pictures: []*dto.FetchedPicture{
			{
				Index:     0,
				Thumbnail: original,
				Original:  original,
			},
		},
	}
	if item.UserID != "" {
		fetched.Artist = &dto.FetchedArtist{
			Name:     item.UserName,
			Type:     shared.SourceTypePixiv,
			UID:      item.UserID,
			Username: item.UserAccount,
		}
	}
	return fetched
}
