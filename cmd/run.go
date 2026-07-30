package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/goccy/go-json"
	"github.com/wwwangzilin/LotsACG/internal/app"
	"github.com/wwwangzilin/LotsACG/internal/common/version"
	"github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
	"github.com/wwwangzilin/LotsACG/pkg/log"
	"github.com/unvgo/ouid"
)

const banner = `
  __  __                              _       ____    ____ 
 |  \/  |   __ _   _ __    _   _     / \     / ___|  / ___|
 | |\/| |  / _  | | '_ \  | | | |   / _ \   | |     | |  _ 
 | |  | | | (_| | | | | | | |_| |  / ___ \  | |___  | |_| |
 |_|  |_|  \__,_| |_| |_|  \__, | /_/   \_\  \____|  \____|
                           |___/                                        

Build time: %s  Version: %s  Commit: %s
Github: https://github.com/wwwangzilin/LotsACG
Kawaii is All You Need! ᕕ(◠ڼ◠)ᕗ

`

func Run() {
	fmt.Printf(banner, version.BuildTime, version.Version, version.Commit[:7])
	ouid.MarshalJSON = json.Marshal
	ouid.UnmarshalJSON = json.Unmarshal

	cfg := runtimecfg.Get()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, stop); err != nil {
		log.Fatal(err)
	}
}
