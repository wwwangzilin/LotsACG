package runtimecfg

// XPPusherConfig 配置内置管理的 Pixiv-XP-Pusher (Python) 进程。
type XPPusherConfig struct {
	// Dir XP-Pusher 项目目录 (含 main.py)
	Dir string `toml:"dir" mapstructure:"dir" json:"dir" yaml:"dir"`
	// Python Python 解释器路径; 为空时自动使用 <dir>/.venv/Scripts/python.exe (Windows) 或 <dir>/.venv/bin/python
	Python string `toml:"python" mapstructure:"python" json:"python" yaml:"python"`
	// Command 主脚本文件名, 默认 main.py
	Command string `toml:"command" mapstructure:"command" json:"command" yaml:"command"`
	// Args 附加参数 (空格分隔), 如 "--once"
	Args string `toml:"args" mapstructure:"args" json:"args" yaml:"args"`
}
