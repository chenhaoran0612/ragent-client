package config

import (
	"io/ioutil"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Config struct {
	ListenIp            string   `yaml:"ListenIp"`
	ListenPort          int      `yaml:"ListenPort"`
	RpcPort             int      `yaml:"RpcPort"`
	PerformancePort     int      `yaml:"PerformancePort"`
	Servers             []Server `yaml:"Server"`
	IsAutoTransServer   bool     `yaml:"IsAutoTransServer"`
	AutoTransServerDual int      `yaml:"AutoTransServerDual"`
	Control             string   `yaml:"Control"`
	ServerId            uint64   `yaml:"ServerId"`
	Logger              Logger   `yaml:"logger"`
	Redis               Redis    `yaml:"redis"`
}

type Redis struct {
	Address       string `yaml:"address"`
	Password      string `yaml:"password"`
	Port          string `yaml:"port"`
	OpMaxIdle     int    `yaml:"op_maxidle"`
	OpMaxActive   int    `yaml:"op_maxactive"`
	OpIdleTimeout int    `yaml:"op_idletimeout"`
}

type Logger struct {
	Filename   string `yaml:"filename"`
	MaxSize    int    `yaml:"maxSize"`
	MaxBackups int    `yaml:"maxBackups"`
	MaxAge     int    `yaml:"maxAge"`
	Level      string `yaml:"level"`
	Verbose    int    `yaml:"verbose"`
	Compress   bool   `yaml:"compress"`
}

type Server struct {
	Ip   string `yaml:"ip"`
	Port int    `yaml:"port"`
}

func GetDefaultConfig() *Config {
	return &Config{
		ListenIp:        "0.0.0.0",
		ListenPort:      3333,
		RpcPort:         9090,
		PerformancePort: 9000,
		Servers: []Server{
			{
				Ip:   "hk.agent.com",
				Port: 8888,
			},
			{
				Ip:   "local.agent.com",
				Port: 8888,
			},
		},
		IsAutoTransServer:   false,
		AutoTransServerDual: 3600,
		Control:             "agent.test.com:5678",
		ServerId:            1,
		Logger: Logger{
			Filename:   "./logs/ragent-client.log",
			MaxSize:    1000,
			MaxBackups: 100,
			MaxAge:     24 * 30,
			Level:      "error",
			Verbose:    3,
			Compress:   true,
		},
		Redis: Redis{
			Address:       "127.0.0.1",
			Password:      "agentRedis",
			Port:          "6379",
			OpMaxIdle:     3,
			OpMaxActive:   10,
			OpIdleTimeout: 0,
		},
	}
}

func LoadConfig(path string) (config *Config) {

	if path == "" {
		zap.L().Fatal("Please set startup parameters-configs or RAGETNT_CONFIG_PATH environment variable", zap.String("path", path))
	}
	configContent, err := ioutil.ReadFile(path)

	if err != nil {
		zap.L().Fatal("read config file error: ", zap.String("path", path))
	}

	err = yaml.Unmarshal(configContent, &config)
	if err != nil {
		zap.L().Fatal("config file can't be decode as json: ", zap.Error(err))
	}
	if config.ListenPort <= 0 {
		zap.L().Fatal("config file error: ListenPort", zap.Int("ListenPort", config.ListenPort))
	}

	if config.RpcPort <= 0 {
		zap.L().Fatal("config file error: RpcPort", zap.Int("RpcPort", config.RpcPort))
	}

	if len(config.Logger.Filename) <= 0 {
		zap.L().Fatal("config file error: Logger Filename is null")
	}

	if config.Logger.MaxSize == 0 {
		config.Logger.MaxSize = 1000
	}

	if config.Logger.MaxBackups == 0 {
		config.Logger.MaxBackups = 100
	}

	if config.Logger.MaxAge == 0 {
		config.Logger.MaxAge = 24 * 30
	}

	if len(config.Logger.Level) <= 0 {
		config.Logger.Level = "info"
	}

	if config.Logger.Verbose == 0 {
		config.Logger.Verbose = 3
	}
	return
}

const (
	CONFIG_PATH = "RAGETNT_CONFIG_PATH"
	LOG_DIR     = "RAGETNT_LOG_DIR"
)
