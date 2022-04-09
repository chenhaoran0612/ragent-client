package main

import (
	"amenity.com/ragent-client/config"
	"amenity.com/ragent-client/service/client"
	service "amenity.com/ragent-client/service/rpc"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	var (
		configPath, logDir string
		defaultConfigFlag  string
	)

	flag.StringVar(&configPath, "config", "./config/dev/config.dev.yaml", "Path of config file")
	flag.StringVar(&logDir, "log_dir", "./log/", "logger directory.")
	flag.StringVar(&defaultConfigFlag, "default_config", "", "logger directory.")
	flag.Parse()
	if configPath == "" && defaultConfigFlag != "true" {
		configPath = os.Getenv(config.CONFIG_PATH)
	}
	// runtimeFilePath := flag.String("runtime", "", "Path of runtime file, use for zero downtime upgrade.")
	if logDir == "" {
		logDir = os.Getenv(config.LOG_DIR)
	}
	gS := client.NewClientInstance(configPath, defaultConfigFlag, logDir)

	undo := zap.ReplaceGlobals(gS.ZapLog.Logger)
	defer undo()

	zap.L().Info("init logger", zap.String("logName", gS.Config.Logger.Filename), zap.String("logDir", logDir), zap.Int("noNeedVerbose", gS.Config.Logger.Verbose))

	// kit.G(func() {
	go gS.Run()

	// 处理自动切换矿池操作 server挂 | 到切换时间
	go gS.AutoTransServer(gS.Config.IsAutoTransServer)
	// })

	rS := service.NewRpcService(gS.Config.RpcPort)
	go rS.Run()

	// go 性能分析工具
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", gS.Config.PerformancePort), nil)
	}()

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGUSR2)

	select {
	case sig := <-signalChan:

		switch sig {
		case syscall.SIGUSR2:
			zap.L().Info("now upgrading")
			err := gS.UpgradeSvc.Upgrade()
			if err != nil {
				zap.L().Warn("upgrade failed", zap.Error(err))
			}
			break
		default:
			zap.L().Info("now exiting", zap.String("sig", sig.String()))
		}
		break
	}
}
