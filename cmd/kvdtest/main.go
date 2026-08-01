package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
)

type session struct {
	CurrentSourceURL string
	CurrentTags      []string
}

type pref struct {
	PositiveWeights map[string]float64
	TagPairs        map[string]float64
}

func main() {
	dir, _ := os.MkdirTemp("", "kvdtest")
	defer os.RemoveAll(dir)
	kvstor.Init(runtimecfg.KVDBConfig{
		Type:           "bbolt",
		Path:           filepath.Join(dir, "test.kv"),
		Bucket:         "kv",
		TTLBucket:      "kv_ttl",
		TTLBatchLimit:  1024,
		TTLSweepPeriod: 60,
	})
	defer kvstor.Close()
	ctx := context.Background()

	// struct
	s := &session{CurrentSourceURL: "https://www.pixiv.net/artworks/123", CurrentTags: []string{"a", "b"}}
	if err := kvstor.SetWithTTL(ctx, "s1", s, 7*24*time.Hour); err != nil {
		fmt.Println("set struct err:", err)
	}
	got, err := kvstor.Get[session](ctx, "s1")
	fmt.Printf("struct: err=%v url=%q tags=%v\n", err, got.CurrentSourceURL, got.CurrentTags)

	// map
	mp := map[string]float64{"t1": 1.5, "t2": 2.5}
	if err := kvstor.Set(ctx, "m1", mp); err != nil {
		fmt.Println("set map err:", err)
	}
	gm, err := kvstor.Get[map[string]float64](ctx, "m1")
	fmt.Printf("map: err=%v val=%v\n", err, gm)

	// pointer struct (UserPreference style)
	p := &pref{PositiveWeights: map[string]float64{"x": 3}, TagPairs: map[string]float64{"a|b": 1}}
	if err := kvstor.SetWithTTL(ctx, "p1", p, time.Hour); err != nil {
		fmt.Println("set ptr err:", err)
	}
	gp, err := kvstor.Get[*pref](ctx, "p1")
	fmt.Printf("ptr: err=%v val=%+v\n", err, gp)

	// string
	if err := kvstor.Set(ctx, "str1", "hello"); err != nil {
		fmt.Println("set str err:", err)
	}
	gs, err := kvstor.Get[string](ctx, "str1")
	fmt.Printf("string: err=%v val=%q\n", err, gs)

	// missing key
	_, err = kvstor.Get[session](ctx, "missing")
	fmt.Printf("missing: err=%v\n", err)

	// TTL expired
	if err := kvstor.SetWithTTL(ctx, "ttl1", "v", 30*time.Millisecond); err != nil {
		fmt.Println("set ttl err:", err)
	}
	time.Sleep(70 * time.Millisecond)
	_, err = kvstor.Get[string](ctx, "ttl1")
	fmt.Printf("ttl expired: err=%v\n", err)
}
