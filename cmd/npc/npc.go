package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ehang.io/nps/client"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego/logs"
	"github.com/ccding/go-stun/stun"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/kardianos/service"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

var (
	serverAddr     = flag.String("server", "proxy.cupb.top:8024", "Server addr (ip:port)")
	configPath     = flag.String("config", "", "Configuration file path")
	verifyKey      = flag.String("vkey", "", "Authentication key")
	logType        = flag.String("log", "stdout", "Log output mode（stdout|file）")
	connType       = flag.String("type", "tcp", "Connection type with the server（kcp|tcp）")
	proxyUrl       = flag.String("proxy", "", "proxy socks5 url(eg:socks5://111:222@127.0.0.1:9007)")
	logLevel       = flag.String("log_level", "2", "log level 0~7") // 需要显示日志改成 7 参考 logs.LevelDebug
	registerTime   = flag.Int("time", 2, "register time long /h")
	localPort      = flag.Int("local_port", 2000, "p2p local port")
	password       = flag.String("password", "", "p2p password flag")
	target         = flag.String("target", "", "p2p target")
	localType      = flag.String("local_type", "p2p", "p2p target")
	logPath        = flag.String("log_path", "", "npc log path")
	debug          = flag.Bool("debug", true, "npc debug")
	pprofAddr      = flag.String("pprof", "", "PProf debug addr (ip:port)")
	stunAddr       = flag.String("stun_addr", "stun.stunprotocol.org:3478", "stun server address (eg:stun.stunprotocol.org:3478)")
	ver            = flag.Bool("version", false, "show current version")
	disconnectTime = flag.Int("disconnect_timeout", 60, "not receiving check packet times, until timeout will disconnect the client")
)

func main() {

	flag.Parse()
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	//logs.SetLevel(logs.LevelCritical) // 设置日志级别为 Critical ，不需要输出日志，避免磁盘满了。  上面log_level 改成2即可
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
	if *debug {
		logs.SetLogger(logs.AdapterConsole, `{"level":`+*logLevel+`,"color":true}`)
	} else {
		logs.SetLogger(logs.AdapterFile, `{"level":`+*logLevel+`,"filename":"`+*logPath+`","daily":false,"maxlines":100000,"color":true}`)
	}

	if false {
		// 目前存在交叉获取对方进程,然后都结束的情况.并发不安全
		if currentIsRunning() {
			logs.Critical("已经运行了")
			os.Exit(0)
		}
	}

	// init service
	options := make(service.KeyValue)
	svcConfig := &service.Config{
		Name:        "Kuaima IP",
		DisplayName: "快码ip地址修改",
		Description: "快码ip地址修改客户端，联系微信 aohanhongyi",
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

	// 如果是给arm开发板使用，就不需要下面这么多注册服务啥的，直接false即可。 一般电脑就是true，注册服务。
	//  尝试调用另一个文件，后缀用arm和非arm来区分即可决定函数在编译时候的执行。这样就不会改了
	// Determine the architecture
	arch := runtime.GOARCH

	if arch == "arm" || arch == "arm64" {
		logs.Info("Running on ARM architecture")
		// ARM-specific code here
	} else {
		logs.Info("Running on non-ARM architecture")
		// Non-ARM-specific code here
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
		} else {
			// 这一步只会在初始化的时候右键，管理员运行，之后再也不会用了，因为后期服务注册都会直接带上了参数。走上面了。
			// 从txt文件里读取verifyKey
			fileName := "keyFile.txt"
			if len(*verifyKey) == 0 {
				_, err := os.Stat(fileName)
				if err != nil {
					logs.Error("文件%v不存在 %v", fileName, err)
				}
				if os.IsNotExist(err) {
					file1, err1 := os.Create(fileName)
					//写入文件
					n, err1 := file1.WriteString(*verifyKey)
					if err1 != nil {
						logs.Error("文件写入失败 %s %s", fileName, err)
					} else {
						logs.Info("%v 文件初始化创建结果 %v", fileName, n)
					}
				} else {
					fileContent, fileErr := os.ReadFile(fileName)
					if fileErr == nil && len(fileContent) > 0 {
						fileContentString := string(fileContent)
						fileContentString = strings.Replace(fileContentString, "\r", "", -1)
						fileContentString = strings.Replace(fileContentString, "\n", "", -1)
						fileContentString = strings.TrimSpace(fileContentString)
						*verifyKey = fileContentString
					}
				}
			}
			if len(*verifyKey) == 0 {
				logs.Error("verifyKey不能为空,地址 %v", fileName)
				os.Exit(0)
			}

			svcConfig.Arguments = append(svcConfig.Arguments, "-vkey="+*verifyKey)
			msg := fmt.Sprintf("初始化，没有参数，默认注册安装,key:%v", *verifyKey)
			logs.Info(msg)
			// 如果没有参数默认就注册
			service.Control(s, "stop")
			service.Control(s, "uninstall")
			//install.InstallNpc()
			err4 := service.Control(s, "install")
			if err4 != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err4.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				confPath := "/etc/init.d/" + svcConfig.Name
				os.Symlink(confPath, "/etc/rc.d/S90"+svcConfig.Name)
				os.Symlink(confPath, "/etc/rc.d/K02"+svcConfig.Name)
			}
		}
		// 创建都启动器快捷方式
		createShortcut()
	}
	s.Run()
}

// 创建快捷方式
func createShortcut() {
	// 判断程序当前是否在桌面，不在桌面就创建快捷方式。 因为客服下载后，多半在下载文件夹。

	if true {
		// Get the path of the current executable
		exePath, err := os.Executable()
		if err != nil {
			logs.Error(err)
		}
		logs.Info("当前执行程序是 %v", exePath)

		// Initialize OLE
		ole.CoInitialize(0)
		defer ole.CoUninitialize()

		// Create a COM object for WScript.Shell
		unknown, err := oleutil.CreateObject("WScript.Shell")
		if err != nil {
			logs.Error(err)
			return
		}
		defer unknown.Release()
		// Query for the IDispatch interface
		shell, err := unknown.QueryInterface(ole.IID_IDispatch)
		if err != nil {
			logs.Error(err)
			return
		}
		defer shell.Release()

		// Get the desktop folder path
		startupPath := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
		//desktopPath := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
		shortcutPath := filepath.Join(startupPath, "快码ip-service.lnk")

		stat, err2 := os.Stat(shortcutPath)
		if !os.IsNotExist(err2) {

			logs.Info("%v", stat)
			// 判断是否是当前程序的，
			// Load the existing shortcut
			shortcut, err := oleutil.CallMethod(shell, "CreateShortcut", shortcutPath)
			if err != nil {
				logs.Error(err)
			}
			shortcutDisp := shortcut.ToIDispatch()
			defer shortcutDisp.Release()

			// Get the TargetPath of the existing shortcut
			existingTargetPath, err := oleutil.GetProperty(shortcutDisp, "TargetPath")
			if err != nil {
				logs.Error(err)
			}
			existingTargetPathStr := existingTargetPath.ToString()

			// Compare the existing TargetPath with the current executable path
			if existingTargetPathStr == exePath {
				// 如果当前快捷方式与自己的是一个启动程序，那么没必要安装了。
				logs.Info("The shortcut already points to the current application. No need to overwrite.")
				return
			} else {
				logs.Info("The shortcut points to a different application. It will be overwritten.")
			}
		}

		{

			// Create the shortcut COM object
			shortcut, err := oleutil.CallMethod(shell, "CreateShortcut", shortcutPath)
			if err != nil {
				logs.Error(err)
			}
			shortcutDisp := shortcut.ToIDispatch()
			if shortcutDisp != nil {

				defer shortcutDisp.Release()

				// Set the TargetPath for the shortcut
				_, err = oleutil.PutProperty(shortcutDisp, "TargetPath", exePath)
				if err != nil {
					logs.Error(err)
				}

				// Optionally, set the WorkingDirectory
				_, err = oleutil.PutProperty(shortcutDisp, "WorkingDirectory", filepath.Dir(exePath))
				if err != nil {
					logs.Error(err)
				}

				// Optionally, set the shortcut's description
				_, err = oleutil.PutProperty(shortcutDisp, "Description", "快码改ip Service")
				if err != nil {
					logs.Error(err)
				}

				// Optionally, set the icon for the shortcut (use the application's icon)
				_, err = oleutil.PutProperty(shortcutDisp, "IconLocation", exePath)
				if err != nil {
					logs.Error(err)
				}

				// Save the shortcut
				_, err = oleutil.CallMethod(shortcutDisp, "Save")
				if err != nil {
					logs.Error(err)
				}
				logs.Info("Shortcut created successfully on the desktop:", shortcutPath)
			} else {
				logs.Error("创建快捷方式失败")
			}
		}
	}
}

func currentIsRunning() bool {

	// Get the path of the current executable
	exePath, err := os.Executable()
	if err != nil {
		logs.Error(err)
	}
	logs.Info("当前执行程序是1 %v", exePath)

	exeName := filepath.Base(exePath) // Replace with your process name

	processName := "kuaima"

	// Command to get the list of all running processes
	cmd := exec.Command("tasklist")
	var out bytes.Buffer
	cmd.Stdout = &out

	// 静默执行cmd Hide the console window when executing the command
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	err1 := cmd.Run()
	if err1 != nil {
		fmt.Println("Error running command:", err1)
		return false
	}

	// Convert the output from the Windows encoding to UTF-8
	decodedOutput, err := decodeWindows1252(out.Bytes())
	if err != nil {
		fmt.Println("Error decoding output:", err)
		return false
	}

	output := string(decodedOutput)

	// Split the output into lines
	taskLines := strings.Split(output, "\n")

	for _, taskLine := range taskLines {
		// 所以打包的名字必须含有kuaima和ip
		if strings.Contains(taskLine, exeName) || (strings.Contains(taskLine, processName) && strings.Contains(taskLine, "ip")) {
			// Extract PID
			fields := strings.Fields(taskLine)
			if len(fields) > 1 {
				logs.Info("%s is running.\n", fields[0])
				pid := fields[1]
				logs.Info("Process ID (PID): %s\n", pid)
				atoi, err12 := strconv.Atoi(pid)
				if err12 != nil {
					logs.Error(err12)
				} else {
					getpid := os.Getpid()
					if atoi == getpid {
						return false
					} else {

						// 启动gin来持有端口号.  2357
						// 定义一个处理 GET 请求的处理器函数
						http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
							if r.Method == http.MethodGet {
								logs.Info(w, "Hello, World!")
							} else {
								w.WriteHeader(http.StatusMethodNotAllowed)
								logs.Info(w, "Method not allowed")
							}
						})

						if false {
							// 这个要求弹窗允许网络,多了一步.
							// 启动 HTTP 服务器，监听在 8080 端口
							logs.Info("Starting server at port 2357")
							if err := http.ListenAndServe(":2357", nil); err != nil {
								logs.Error("Could not start server: %s\n", err)
								os.Exit(1)
							}
						}

						// TODO 进一步查询服务器，看看是否在线
						return true
					}
				}
			}
			return false
		}
	}

	return false
}

// decodeWindows1252 converts Windows-1252 encoded bytes to UTF-8
func decodeWindows1252(input []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(input), charmap.Windows1252.NewDecoder())
	return io.ReadAll(reader)
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
				logs.Info("Client closed! It will be reconnected in five seconds")
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
