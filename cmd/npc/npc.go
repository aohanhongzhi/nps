package main

import (
	"ehang.io/nps/client"
	"ehang.io/nps/jtrpc"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/version"
	"flag"
	"fmt"
	"github.com/ccding/go-stun/stun"
	"github.com/kardianos/service"
	logs "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	serverAddr = flag.String("server", "", "Server addr (ip:port)")
	configPath = flag.String("config", "", "Configuration file path")
	verifyKey  = flag.String("vkey", "", "Authentication key")
	//logType        = flag.String("log", "stdout", "Log output mode（stdout|file）")
	connType = flag.String("type", "tcp", "Connection type with the server（kcp|tcp）")
	proxyUrl = flag.String("proxy", "", "proxy socks5 url(eg:socks5://111:222@127.0.0.1:9007)")
	//logLevel       = flag.String("log_level", "7", "log level 0~7")
	registerTime = flag.Int("time", 2, "register time long /h")
	localPort    = flag.Int("local_port", 2000, "p2p local port")
	password     = flag.String("password", "", "p2p password flag")
	target       = flag.String("target", "", "p2p target")
	localType    = flag.String("local_type", "p2p", "p2p target")
	logPath      = flag.String("log_path", "", "npc log path")
	//debug          = flag.Bool("debug", true, "npc debug")
	pprofAddr      = flag.String("pprof", "", "PProf debug addr (ip:port)")
	stunAddr       = flag.String("stun_addr", "stun.stunprotocol.org:3478", "stun server address (eg:stun.stunprotocol.org:3478)")
	ver            = flag.Bool("version", false, "show current version")
	disconnectTime = flag.Int("disconnect_timeout", 60, "not receiving check packet times, until timeout will disconnect the client")
)

func main() {
	flag.Parse()
	//logs.Reset()
	//logs.EnableFuncCallDepth(true)
	//logs.SetLogFuncCallDepth(3)
	if *ver {
		common.PrintVersion()
		return
	}
	if *logPath == "" {
		*logPath = common.GetNpcLogPath()
	}
	if common.IsWindows() {
		*logPath = strings.Replace(*logPath, "\\", "\\\\", -1)
	}
	//if *debug {
	//	logs.SetLogger(logs.AdapterConsole, `{"level":`+*logLevel+`,"color":true}`)
	//} else {
	//	logs.SetLogger(logs.AdapterFile, `{"level":`+*logLevel+`,"filename":"`+*logPath+`","daily":false,"maxlines":100000,"color":true}`)
	//}

	// init service
	options := make(service.KeyValue)
	svcConfig := &service.Config{
		Name:        "corss-ip",
		DisplayName: "corss ip地址修改客户端",
		Description: "ip地址修改客户端",
		Option:      options,
	}
	if !common.IsWindows() {
		svcConfig.Dependencies = []string{
			"Requires=network.target",
			"After=network-online.target syslog.target"}
		svcConfig.Option["SystemdScript"] = install.SystemdScript
		svcConfig.Option["SysvScript"] = install.SysvScript
	}
	for _, v := range os.Args[1:] {
		switch v {
		case "install", "start", "stop", "uninstall", "restart":
			continue
		}
		if !strings.Contains(v, "-service=") && !strings.Contains(v, "-debug=") {
			svcConfig.Arguments = append(svcConfig.Arguments, v)
		}
	}
	svcConfig.Arguments = append(svcConfig.Arguments, "-debug=false")
	prg := &npc{
		exit: make(chan struct{}),
	}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logs.Error(err, "service function disabled")
		run()
		// run without service
		wg := sync.WaitGroup{}
		wg.Add(1)
		wg.Wait()
		return
	}
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "status":
			if len(os.Args) > 2 {
				path := strings.Replace(os.Args[2], "-config=", "", -1)
				client.GetTaskStatus(path)
			}
		case "register":
			flag.CommandLine.Parse(os.Args[2:])
			client.RegisterLocalIp(*serverAddr, *verifyKey, *connType, *proxyUrl, *registerTime)
		case "update":
			install.UpdateNpc()
			return
		case "nat":
			c := stun.NewClient()
			c.SetServerAddr(*stunAddr)
			nat, host, err := c.Discover()
			if err != nil || host == nil {
				logs.Error("get nat type error", err)
				return
			}
			fmt.Printf("nat type: %s \npublic address: %s\n", nat.String(), host.String())
			os.Exit(0)
		case "start", "stop", "restart":
			// support busyBox and sysV, for openWrt
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				cmd := exec.Command("/etc/init.d/"+svcConfig.Name, os.Args[1])
				err := cmd.Run()
				if err != nil {
					logs.Error(err)
				}
				return
			}
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			return
		case "install":
			service.Control(s, "stop")
			service.Control(s, "uninstall")
			install.InstallNpc()
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				confPath := "/etc/init.d/" + svcConfig.Name
				os.Symlink(confPath, "/etc/rc.d/S90"+svcConfig.Name)
				os.Symlink(confPath, "/etc/rc.d/K02"+svcConfig.Name)
			}
			return
		case "uninstall":
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				os.Remove("/etc/rc.d/S90" + svcConfig.Name)
				os.Remove("/etc/rc.d/K02" + svcConfig.Name)
			}
			return
		}
	}

	go jtrpc.RpcServer()

	s.Run()
}

type npc struct {
	exit chan struct{}
}

func (p *npc) Start(s service.Service) error {
	go p.run()
	return nil
}
func (p *npc) Stop(s service.Service) error {
	close(p.exit)
	if service.Interactive() {
		os.Exit(0)
	}
	return nil
}

func (p *npc) run() error {
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logs.Warning("npc: panic serving %v: %v\n%s", err, string(buf))
		}
	}()
	run()
	select {
	case <-p.exit:
		logs.Warning("stop...")
	}
	return nil
}

func run() {
	common.InitPProfFromArg(*pprofAddr)

	*serverAddr = "proxy.cupb.top:8024"
	//*serverAddr = "localhost:8024"
	//*verifyKey = "ozsathdqpjilcbs0"
	*verifyKey = "vkdcivcm2i99rpn3" //邢台
	*verifyKey = "n7dvxnh1bd3zjlj6" //邢台
	*verifyKey = "4szvsq9ojbycyyuv" //河北省张家口市万全区
	*verifyKey = "vkdcivcm2i99rpn3" //邢台
	*verifyKey = "5myyool4o5wl6m0w" //白沟
	*verifyKey = "1zx6rnppw3elxlet" // 合肥
	*verifyKey = "z5dy613iymrnb2q3" // 石家庄阿拉蕾
	*verifyKey = "dn0qbr72ufiassvx" // 河北邢台柏乡
	*verifyKey = "td5nh1newzjpm1pi" // 河北邢台柏乡
	*verifyKey = "hr8mvx8b9e199f4y" // 河北邢台柏乡
	*verifyKey = "e4l7tmd3yrjwp3a5" // 	河北沧州新华区
	*verifyKey = "xyxg47irenhxraqo" // 河北邢台顺德路网点
	*verifyKey = "y99lmqhzx2cub15e" // 河北保定市安国市
	*verifyKey = "nvj3u6wjgaqi6x3a" // 合肥2
	//*verifyKey = "1k1yj39r0cedmy2o" // mac

	*verifyKey = "3p2uhlwv98idgz89" //Alienware
	*verifyKey = "7j0jl5b1rxlbme3i" //河北省保定市安国市-公司电脑
	*verifyKey = "a59r8i9nbuc52xdh" //河北省保定市安国市-公司电脑
	*verifyKey = "kuy2eqpoi4cli7v6" //河北省承德市宽城满族自治县
	*verifyKey = "vomz36e6pivgabgj" //河北省沧州市沧县
	*verifyKey = "0cozs7zbkgb89gzv" //gaoyangxian
	*verifyKey = "3e587gf8bipxy0cd" //	河北省张家口市万全区
	*verifyKey = "uudzpaalldhfj157" //	河北省衡水桃城区武邑县城网点
	*verifyKey = "tt5kj68lpdwajpww" //		衡水枣强县城网点
	*verifyKey = "5myyool4o5wl6m0w" //白沟
	*verifyKey = "zak7y1o9kf7ejzkz" //白沟新
	*verifyKey = "fplqd0vs89mlhfn0" //沧州仲裁员
	*verifyKey = "a42dbebo0c8h9421" //	沧州2
	*verifyKey = "i3w1m2p88pno2kmf" //  沧州公司电脑
	*verifyKey = "tabfdgmg9j8bngj0" //  沧州-仲裁员-星星
	*verifyKey = "i9txtts4czczesen" //  衡水市
	*verifyKey = "nkxrnlignmmg8v21" //  	广东省龙岗区
	*verifyKey = "8b7xawrz9qq79lm2" //  	沧州孟姐
	*verifyKey = "z27p0alifallqc57" //  	沧州孟姐
	*verifyKey = "4tfwy83dzssgz7qd" //  	河北省承德市宽城满足自治县
	*verifyKey = "jiyof6fuggvd23jy" //  	河北省保定市莲池区
	*verifyKey = "uudzpaalldhfj157" //  	河北省衡水桃城区武邑县城网点
	*verifyKey = "ekqukhj6zwu8eass" //  		上海市长宁
	*verifyKey = "3znisuvuhssvltzs" //  安徽亳州
	*verifyKey = "coketfsi0cgrvl4t" //  中国黑龙江齐齐哈尔龙沙区-王小二客服
	*verifyKey = "hhpreh0r4zsbfo1p" //  河北省唐山市开平区-王小二-王小二
	*verifyKey = "3p2uhlwv98idgz89" // 我自己的 Alienware
	*verifyKey = "pvgblavi6axm8jvx" // 我自己的 Alienware
	*verifyKey = "508qdipotgryywf0" // 我自己的 Alienware
	*verifyKey = "ahjnjff29p6or701" // 高碑店3
	*verifyKey = "ykvbb45xo8rw0et8" // 白沟新城-公司电脑-杨宇静
	*verifyKey = "hnrs4gy2nwm70pfj" // 	河北省秦皇岛海港区汤河网点-新
	*verifyKey = "wfr7ketbvq04bnxj" // 	秦皇岛青龙县

	logs.Info("当前clientId %v", *verifyKey)

	*connType = "tcp"
	//*target = "localhost:1235"

	//p2p or secret command
	if *password != "" {
		commonConfig := new(config.CommonConfig)
		commonConfig.Server = *serverAddr
		commonConfig.VKey = *verifyKey
		commonConfig.Tp = *connType
		localServer := new(config.LocalServer)
		localServer.Type = *localType
		localServer.Password = *password
		localServer.Target = *target
		localServer.Port = *localPort
		commonConfig.Client = new(file.Client)
		commonConfig.Client.Cnf = new(file.Config)
		go client.StartLocalServer(localServer, commonConfig)
		return
	}
	env := common.GetEnvMap()
	if *serverAddr == "" {
		*serverAddr, _ = env["NPC_SERVER_ADDR"]
	}
	if *verifyKey == "" {
		*verifyKey, _ = env["NPC_SERVER_VKEY"]
	}
	logs.Info("the version of client is %s, the core version of client is %s verifyKey:%s configPath:%s", version.VERSION, version.GetVersion(), *verifyKey, *configPath)
	if *verifyKey != "" && *serverAddr != "" && *configPath == "" {
		go func() {
			for {
				client.NewRPClient(*serverAddr, *verifyKey, *connType, *proxyUrl, nil, *disconnectTime).Start()
				logs.Info("Client closed! It will be reconnected in five seconds ,serverAddr:%s,verifyKey:%s", *serverAddr, *verifyKey)
				time.Sleep(time.Second * 5)
			}
		}()
	} else {
		if *configPath == "" {
			*configPath = common.GetConfigPath()
		}
		go client.StartFromFile(*configPath)
	}
}
