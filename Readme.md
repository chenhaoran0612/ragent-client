# RAgent Client - Golang 版本

## quick start

* 首先是拉代码, 编译, 以及获取依赖; 这部分因为部署需求以及服务器环境不同, 需要灵活变通, 这里给一个开发环境的流程

```shell
# 测试环境部署需要设置代理
go env -w GOPROXY=https://goproxy.cn,direct
# 获取依赖
go get
# 编译(要根据实际需求,添加和修改编译参数,这里就不列了)
go build .
# 运行
./new-agent-client
```

* 启动参数说明

>启动项目需要指定配置文件路径, 有三种方式实现这一点
>
>项目/config/dev/config.dev.yaml 是开发使用的默认配置文件

```shell
# 1.通过配置环境变量来完成配置文件以及日志文件路径的指定
export RAGETNT_CONFIG_PATH = "配置文件路径(到文件)"
export RAGETNT_LOG_DIR = "日志文件路径(到目录即可)"

# 2.通过启动参数指定目录
./agent -config "配置文件路径(到文件级别)" -log_dir "日志目录"

# 3.通过默认配置启动(集成无外置配置文件, 需要到 项目/config/config.go 的 GetDefaultConfig() 方法对默认配置进行修改)
./agent -default_config true
```

* 运行检查

> 启动后监控日志来查看项目运行情况, 也可以通过配置文件的健康检查端口来确认
> 
> 如果项目启动后自动停止, 大概率是由于没有找到配置文件造成的
> 
> 其他问题可以参考日志输出的内容来排查


## 配置文件详解

> 配置文件分为集成配置文件和外置配置文件两种, 为方便windows部署去掉了配置文件的注释, 这里分别介绍其中的参数含义

* 外置配置文件, 示例在 项目路径/config/dev/config.dev.yaml

```yarm
ListenIp: 0.0.0.0                       # 服务使用的IP
ListenPort: 12000                       # 监听端口号(矿机连接的端口号)
RpcPort : 12581                         # Http 健康检测端口
Control : localhost:80                  # 控制 client 预留未来可能会做web控制页面的地址
PerformancePort : 12582                 # 性能分析端口
# 连接 agent server 的地址, ip 必须配置域名
Server:
    -
      ip: agent.test.com                # 必须配置域名
      port: 5678                        # server 端口
    -                                   # 多 server 配置示例
      ip: agent2.test.com
      port: 5678
IsAutoTransServer: false                # 是否自动切换IP
AutoTransServerDual: 100                # 自动切换IP频率 单位:S
ServerId : 3
logger:
  filename: ./agent_client.log          # 客户端日志名称, 目前已修复可用 
  maxSize: 500
  maxBackups: 1000
  maxAge: 1000
  level: debug
  verbose: 1
  compress: true
```

* 内置配置文件, 原理是将配置写到 .go 文件中, 随程序编译打包成可执行文件, 每次修改配置文件需要重新编译才能生效, 好处是无需外置配置文件运行程序. 示例在 项目路径/config/dev/config.go 的 GetDefaultConfig() 方法

```go
func GetDefaultConfig() *Config {
	return &Config{
		ListenIp:            "0.0.0.0",				// 监听矿机ip
		ListenPort:          5678,  			  	// 监听矿机连接的端口号
		RpcPort:             12581,					// 健康检查rpc端口
		PerformancePort:     12582,					// 性能分析端口
		Servers:             []Server{				// server 列表, 下一行的一个大括号代表一个server
			{
				Ip:   "hk.agent.com",				// server 地址,因为用了tls, 这里需要是域名
				Port: 8888,							// server 端口号
			},
            {                                       // 可以继续追加server
                Ip:   "hk2.agent.com",				
                Port: 8888,							
            },
		},
		IsAutoTransServer:   false,					// 是否定时随机切换 true false
		AutoTransServerDual: 0,						// 自动切换server周期 单位:s
		Control:             "",					// 预留未来做 web 端用户控制界面的url
		ServerId:            3,						// 我也不知道有啥用
		Logger: Logger{							    // 日志配置
			Filename:   "./ragent-client.log",		// 日志文件名,但好像不生效
			MaxSize:    1000,
			MaxBackups: 100,
			MaxAge:     24*30,
			Level:      "debug",                    // 日志级别, 可选 debug info error, 越往后打的日志越少
			Verbose:    3,
			Compress:   true,
		},
	}
}
```