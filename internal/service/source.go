package service

import (
	"context"
	"maps"
	"regexp"
	"strings"

	"github.com/wwwangzilin/LotsACG/internal/infra/source"
	"github.com/wwwangzilin/LotsACG/internal/model/dto"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/wwwangzilin/LotsACG/pkg/strutil"
	"github.com/samber/oops"
)

var urlRegex = regexp.MustCompile(`https?://[^\s<>"']+`)

func (s *Service) Source(sourceType shared.SourceType) source.ArtworkSource {
	return s.sources[sourceType]
}

func (s *Service) FindSourceURL(text string) string {
	urls := s.FindSourceURLs(text)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func (s *Service) FindSourceURLs(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")

	var urls []string
	seen := make(map[string]struct{})
	for _, raw := range urlRegex.FindAllString(text, -1) {
		candidate := strings.TrimSpace(strings.TrimRight(raw, " \t.,;:!?)]}"))
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		if s.isSupportedURL(candidate) {
			seen[candidate] = struct{}{}
			urls = append(urls, candidate)
		}
	}
	if len(urls) > 0 {
		return urls
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if s.isSupportedURL(trimmed) {
		return []string{trimmed}
	}
	return nil
}

func (s *Service) isSupportedURL(text string) bool {
	for _, sou := range s.sources {
		if _, ok := sou.MatchesSourceURL(text); ok {
			return true
		}
	}
	return false
}

func (s *Service) FetchArtworkInfo(ctx context.Context, sourceURL string) (*dto.FetchedArtwork, error) {
	for _, sou := range s.sources {
		if _, ok := sou.MatchesSourceURL(sourceURL); ok {
			return sou.GetArtworkInfo(ctx, sourceURL)
		}
	}
	return nil, oops.New("no supported source found")
}

func (s *Service) PrettyFileName(artwork shared.ArtworkLike, picture shared.PictureLike) string {
	for _, sou := range s.sources {
		if _, ok := sou.MatchesSourceURL(artwork.GetSourceURL()); ok {
			return sou.PrettyFileName(artwork, picture)
		}
	}
	ext, _ := strutil.GetFileExtFromURL(picture.GetOriginal())
	return strings.ToLower(strutil.MD5Hash(picture.GetOriginal())) + ext
}

func (s *Service) Sources() map[shared.SourceType]source.ArtworkSource {
	return maps.Clone(s.sources)
}
