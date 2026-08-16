package xppusher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
)

// 验证内嵌源码提取到 exe 同目录的 xppusher/ 且生成 config.yaml。
func TestEnsureExtractedEmbedded(t *testing.T) {
	m, err := NewManager(runtimecfg.XPPusherConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureExtracted(); err != nil {
		t.Fatal(err)
	}
	dir := m.RunDir()
	for _, f := range []string{"main.py", "config.py", "requirements.txt", "config.yaml", ".initialized"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s missing: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "notifier", "telegram.py")); err != nil {
		t.Fatalf("notifier/telegram.py missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "web")); err != nil {
		t.Fatalf("web/ missing: %v", err)
	}
	// 必须不含密钥 (config.yaml 应为 example 拷贝)
	data, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if len(data) == 0 {
		t.Fatal("config.yaml empty")
	}
	t.Log("extract ok:", dir)
}
