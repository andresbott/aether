package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/go-bumbu/config"
)

type AppCfg struct {
	Server     serverCfg
	Obs        serverCfg `config:"Observability"`
	Env        Env
	DataDir    string
	Msgs       []Msg
	TaskRunner TaskRunnerCfg
}

type TaskRunnerCfg struct {
	Parallelism    int
	QueueSize      int
	HistorySize    int
	LogDir         string
	TagReadWorkers int
}

type Env struct {
	LogLevel   string
	Production bool
}

type serverCfg struct {
	BindIp string
	Port   int
}

func (c serverCfg) Addr() string {
	if c.BindIp == "" {
		return ":" + strconv.Itoa(c.Port)
	}
	return c.BindIp + ":" + strconv.Itoa(c.Port)
}

type Msg struct {
	Level string
	Msg   string
}

const EnvBarPrefix = "AETHER"

var defaultCfg = AppCfg{
	DataDir: "./data",
	Server: serverCfg{
		BindIp: "",
		Port:   8075,
	},
	Obs: serverCfg{
		BindIp: "",
		Port:   9009,
	},
	Env: Env{
		LogLevel:   "info",
		Production: false,
	},
	TaskRunner: TaskRunnerCfg{
		Parallelism:    2,
		QueueSize:      20,
		HistorySize:    50,
		TagReadWorkers: 0,
	},
}

func getAppCfg(file string) (AppCfg, error) {
	configMsg := []Msg{}
	cfg := AppCfg{}
	_, err := config.Load(
		config.Defaults{Item: defaultCfg},
		config.EnvFile{Path: ".env", Mandatory: false},
		config.CfgFile{Path: file, Mandatory: false},
		config.EnvVar{Prefix: EnvBarPrefix},
		config.Unmarshal{Item: &cfg},
		config.Writer{Fn: func(level, msg string) {
			if level == config.InfoLevel {
				configMsg = append(configMsg, Msg{Level: "info", Msg: msg})
			}
			if level == config.DebugLevel {
				configMsg = append(configMsg, Msg{Level: "debug", Msg: msg})
			}
		}},
	)
	cfg.Msgs = configMsg
	if err != nil {
		return cfg, err
	}

	absPath, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return cfg, fmt.Errorf("failed to get absolute path: %w", err)
	}
	cfg.DataDir = absPath

	return cfg, nil
}
