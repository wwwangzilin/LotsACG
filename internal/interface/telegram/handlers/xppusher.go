package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/unvgo/ouid"
	"github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
	"github.com/wwwangzilin/LotsACG/internal/infra/kvstor"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/service"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/wwwangzilin/LotsACG/pkg/log"
)

const xpPidKey = "xppusher:pid"

// XPPusher 管理 Pixiv-XP-Pusher (Python) 进程。
// 推荐逻辑 100% 由原始 Python 项目负责, 这里只负责启停/状态, 以及生成「推送到群」用的 API Key。
//
//	/xppusher          查看状态
//	/xppusher start    启动
//	/xppusher stop     停止
//	/xppusher restart  重启
//	/xppusher key      生成/显示推送到频道用的 API Key
func XPPusher(ctx *telegohandler.Context, message telego.Message) error {
	serv, err := requireService(ctx)
	if err != nil {
		return err
	}
	if !utils.CheckPermissionInGroup(ctx, serv, message, shared.PermissionSudo) {
		utils.ReplyMessage(ctx, message, "你没有执行此操作的权限")
		return nil
	}
	_, _, args := telegoutil.ParseCommand(message.Text)
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	cfg := runtimecfg.Get().XPPusher
	if cfg.Dir == "" {
		utils.ReplyMessage(ctx, message, "未配置 XP-Pusher 目录。请在 config.toml 添加:\n[xppusher]\ndir = \"D:/projects/xp/Pixiv-XP-Pusher\"")
		return nil
	}

	switch sub {
	case "start":
		pid, err := startXPProcess(ctx, cfg)
		if err != nil {
			utils.ReplyMessage(ctx, message, "启动失败: "+err.Error())
			return nil
		}
		utils.ReplyMessage(ctx, message, fmt.Sprintf("✅ XP-Pusher 已启动 (PID %d)\n推荐逻辑由原始 Python 项目负责, 与本 bot 相互独立。", pid))
		return nil
	case "stop":
		pid, err := stopXPProcess(ctx, cfg)
		if err != nil {
			utils.ReplyMessage(ctx, message, "停止失败: "+err.Error())
			return nil
		}
		if pid == 0 {
			utils.ReplyMessage(ctx, message, "XP-Pusher 未在运行")
		} else {
			utils.ReplyMessage(ctx, message, fmt.Sprintf("✅ 已停止 XP-Pusher (PID %d)", pid))
		}
		return nil
	case "restart":
		_, _ = stopXPProcess(ctx, cfg)
		time.Sleep(time.Second)
		pid, err := startXPProcess(ctx, cfg)
		if err != nil {
			utils.ReplyMessage(ctx, message, "重启失败: "+err.Error())
			return nil
		}
		utils.ReplyMessage(ctx, message, fmt.Sprintf("✅ XP-Pusher 已重启 (PID %d)", pid))
		return nil
	case "key":
		key, err := createXPPushKey(ctx, serv)
		if err != nil {
			utils.ReplyMessage(ctx, message, "生成 API Key 失败: "+err.Error())
			return nil
		}
		utils.ReplyMessage(ctx, message,
			fmt.Sprintf("🔑 已生成「推送到群」API Key:\n<code>%s</code>\n\n请在 XP-Pusher 的 config.yaml 中添加:\n<code>lotsacg:\n  url: \"http://127.0.0.1:%s\"\n  api_key: \"%s\"</code>",
				key, restPort(), key))
		return nil
	default:
		// 状态
		pid := xpRunningPID(ctx)
		if pid == 0 {
			utils.ReplyMessage(ctx, message, "XP-Pusher: 未在运行\n\n用法:\n/xppusher start - 启动\n/xppusher stop - 停止\n/xppusher restart - 重启\n/xppusher key - 生成推送到群用的 API Key")
			return nil
		}
		utils.ReplyMessage(ctx, message, fmt.Sprintf("XP-Pusher: 正在运行 (PID %d)\n\n/xppusher stop 停止 | /xppusher restart 重启", pid))
		return nil
	}
}

// startXPProcess 启动 XP-Pusher 进程并记录 PID。
func startXPProcess(ctx context.Context, cfg runtimecfg.XPPusherConfig) (int, error) {
	if pid := xpRunningPID(ctx); pid != 0 {
		return pid, nil
	}
	cmdName := resolveXPPython(cfg)
	cmdArgs := []string{}
	command := cfg.Command
	if command == "" {
		command = "main.py"
	}
	cmdArgs = append(cmdArgs, command)
	if cfg.Args != "" {
		cmdArgs = append(cmdArgs, strings.Fields(cfg.Args)...)
	}

	execCmd := exec.Command(cmdName, cmdArgs...)
	execCmd.Dir = cfg.Dir
	// 输出重定向到日志文件
	logDir := filepath.Join(cfg.Dir, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(filepath.Join(logDir, "lotsacg_xppusher.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	execCmd.Stdout = logFile
	execCmd.Stderr = logFile
	if runtime.GOOS == "windows" {
		execCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008} // DETACHED_PROCESS
	}
	if err := execCmd.Start(); err != nil {
		logFile.Close()
		return 0, err
	}
	_ = logFile.Close()
	_ = kvstor.Set(ctx, xpPidKey, execCmd.Process.Pid)
	log.Info("xppusher: started", "pid", execCmd.Process.Pid, "dir", cfg.Dir, "python", cmdName)
	return execCmd.Process.Pid, nil
}

// resolveXPPython 选择合适的 Python 解释器:
// 显式配置 > <dir>/.venv 中的 python (需能 import telegram) > 系统 python。
func resolveXPPython(cfg runtimecfg.XPPusherConfig) string {
	if cfg.Python != "" {
		return cfg.Python
	}
	candidates := make([]string, 0, 2)
	if cfg.Dir != "" {
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(cfg.Dir, ".venv", "Scripts", "python.exe"))
		} else {
			candidates = append(candidates, filepath.Join(cfg.Dir, ".venv", "bin", "python"))
		}
	}
	candidates = append(candidates, "python")
	for _, c := range candidates {
		if c != "python" {
			if _, err := os.Stat(c); err != nil {
				continue
			}
		}
		if pythonCanImport(c, "telegram") {
			return c
		}
	}
	return candidates[len(candidates)-1]
}

// pythonCanImport 判断指定 Python 是否能 import 某个模块 (用于挑选可用解释器)。
func pythonCanImport(python, module string) bool {
	cmd := exec.Command(python, "-c", "import "+module)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// stopXPProcess 停止 XP-Pusher 进程。
func stopXPProcess(ctx context.Context, cfg runtimecfg.XPPusherConfig) (int, error) {
	pid := xpRunningPID(ctx)
	if pid == 0 {
		_ = kvstor.Delete(ctx, xpPidKey)
		return 0, nil
	}
	if runtime.GOOS == "windows" {
		if err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run(); err != nil {
			return pid, err
		}
	} else {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
	}
	_ = kvstor.Delete(ctx, xpPidKey)
	log.Info("xppusher: stopped", "pid", pid)
	return pid, nil
}

// xpRunningPID 返回正在运行的 XP-Pusher 进程 PID, 0 表示未运行。
func xpRunningPID(ctx context.Context) int {
	pid, err := kvstor.Get[int](ctx, xpPidKey)
	if err != nil || pid <= 0 {
		return 0
	}
	if !processAlive(pid) {
		_ = kvstor.Delete(ctx, xpPidKey)
		return 0
	}
	return pid
}

// processAlive 判断进程是否存活。
func processAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), strconv.Itoa(pid))
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// createXPPushKey 生成一个带「发布作品」权限的 API Key, 供 XP-Pusher 推送到群使用。
func createXPPushKey(ctx context.Context, serv *service.Service) (string, error) {
	key := "lotsacg_" + ouid.New().Hex()
	if _, err := serv.CreateApiKey(ctx, key, 0, []shared.Permission{shared.PermissionPostArtwork}, "xppusher push"); err != nil {
		return "", err
	}
	return key, nil
}

// restPort 返回 REST 服务端口, 用于生成默认 URL。
func restPort() string {
	addr := runtimecfg.Get().Rest.Addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[i+1:]
	}
	return "8080"
}
