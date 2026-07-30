package infra

import (
	"github.com/krau/LotsACG/internal/infra/source"
	"github.com/krau/LotsACG/internal/infra/source/impls/bilibili"
	"github.com/krau/LotsACG/internal/infra/source/impls/danbooru"
	"github.com/krau/LotsACG/internal/infra/source/impls/kemono"
	"github.com/krau/LotsACG/internal/infra/source/impls/nhentai"
	"github.com/krau/LotsACG/internal/infra/source/impls/pixiv"
	"github.com/krau/LotsACG/internal/infra/source/impls/twitter"
	"github.com/krau/LotsACG/internal/infra/source/impls/yandere"
)

func initSource() {
	nhentai.Init()
	pixiv.Init()
	twitter.Init()
	yandere.Init()
	bilibili.Init()
	danbooru.Init()
	kemono.Init()

	source.InitAll()
}
