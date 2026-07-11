package pixiv

import (
	"context"
	"fmt"
	"net/http"

	config "github.com/krau/ManyACG/internal/infra/config/runtimecfg"
	"github.com/krau/ManyACG/internal/infra/source"
	"github.com/krau/ManyACG/internal/model/dto"
	"github.com/krau/ManyACG/internal/shared"
	"github.com/krau/ManyACG/pkg/log"
	"github.com/krau/ManyACG/pkg/strutil"
	"github.com/samber/oops"

	"github.com/imroc/req/v3"
)

type Pixiv struct {
	reqClients []*req.Client
	cfg        config.SourcePixivConfig
	clientIdx  int
}

func Init() {
	cfg := config.Get().Source.Pixiv
	if cfg.Disable {
		return
	}

	source.Register(shared.SourceTypePixiv, func() source.ArtworkSource {
		cfg := config.Get().Source.Pixiv
		clients := make([]*req.Client, 0, len(cfg.Accounts)+1)
		if len(cfg.Accounts) > 0 {
			for _, account := range cfg.Accounts {
				cookies := make([]*http.Cookie, 0, len(account.Cookies))
				for _, cookie := range account.Cookies {
					cookies = append(cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value})
				}
				c := req.C().ImpersonateChrome().SetCommonCookies(cookies...)
				c = c.SetLogger(log.Default()).EnableDebugLog().SetCommonRetryCount(3)
				if config.Get().Source.Proxy != "" {
					c.SetProxyURL(config.Get().Source.Proxy)
				}
				clients = append(clients, c)
			}
		} else {
			cookies := make([]*http.Cookie, 0, len(cfg.Cookies))
			for _, cookie := range cfg.Cookies {
				cookies = append(cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value})
			}
			c := req.C().ImpersonateChrome().SetCommonCookies(cookies...)
			c = c.SetLogger(log.Default()).EnableDebugLog().SetCommonRetryCount(3)
			if config.Get().Source.Proxy != "" {
				c.SetProxyURL(config.Get().Source.Proxy)
			}
			clients = append(clients, c)
		}
		return &Pixiv{cfg: cfg, reqClients: clients}
	})
}

func (p *Pixiv) FetchNewArtworks(ctx context.Context, limit int) ([]*dto.FetchedArtwork, error) {
	artworks := make([]*dto.FetchedArtwork, 0)
	errs := make([]error, 0)
	for _, url := range p.cfg.RssURLs {
		artworksForURL, err := p.fetchNewArtworksForRSSURL(ctx, url, limit)
		if err != nil {
			errs = append(errs, err)
		}
		artworks = append(artworks, artworksForURL...)
	}
	if len(errs) > 0 {
		return artworks, fmt.Errorf("fetching pixiv encountered %d errors: %v", len(errs), errs)
	}
	return artworks, nil
}

func (p *Pixiv) GetArtworkInfo(ctx context.Context, sourceURL string) (*dto.FetchedArtwork, error) {
	ajaxResp, err := reqAjaxResp(ctx, sourceURL, p.nextClient())
	if err != nil {
		return nil, err
	}
	if ajaxResp.Err {
		return nil, oops.Wrapf(err, "pixiv ajax response error: %s", ajaxResp.Message)
	}
	return ajaxResp.ToArtwork(ctx, p.nextClient(), p.cfg.ImgProxy)
}

func (p *Pixiv) MatchesSourceURL(text string) (string, bool) {
	pid := getPid(text)
	if pid == "" {
		return "", false
	}
	return "https://www.pixiv.net/artworks/" + pid, true
}

func (p *Pixiv) nextClient() *req.Client {
	if len(p.reqClients) == 0 {
		return nil
	}
	if len(p.reqClients) == 1 {
		return p.reqClients[0]
	}
	client := p.reqClients[p.clientIdx]
	p.clientIdx = (p.clientIdx + 1) % len(p.reqClients)
	return client
}

func (p *Pixiv) PrettyFileName(artwork shared.ArtworkLike, picture shared.PictureLike) string {
	pid := getPid(artwork.GetSourceURL())
	ext, _ := strutil.GetFileExtFromURL(picture.GetOriginal())
	if pid != "" {
		return fmt.Sprintf("pixiv_%s_%d%s", pid, picture.GetIndex(), ext)
	}
	return fmt.Sprintf("pixiv_%s%s", strutil.MD5Hash(picture.GetOriginal()), ext)
}
