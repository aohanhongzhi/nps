package jtrpc

import (
	"fmt"
	nested "github.com/antonfisher/nested-logrus-formatter"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func init1() {
	// 参考文章 https://juejin.cn/post/7026912807333888014
	logPath := "./log"
	errorLogPath := "./log/error/"
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		err1 := os.Mkdir(logPath, os.ModePerm)
		if err1 != nil {
			log.Errorf("日志文件夹创建失败")
		}
	}
	if _, err := os.Stat(errorLogPath); os.IsNotExist(err) {
		err1 := os.Mkdir(errorLogPath, os.ModePerm)
		if err1 != nil {
			log.Errorf("Error日志文件夹创建失败")
		}
	}
	logFilePath := filepath.Join(logPath, "go")
	errorlogFilePath := filepath.Join(errorLogPath, "error")

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	log.SetReportCaller(true)

	FileFormatter := &nested.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		NoColors:        true,
		HideKeys:        true,
		NoFieldsSpace:   false,
		FieldsOrder:     []string{"component", "category", "req"},
		CustomCallerFormatter: func(f *runtime.Frame) string {
			return fmt.Sprintf(" (%s:%d)", f.File, f.Line)
		},
	}
	StdoutFormatter := &nested.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		HideKeys:        true,
		NoFieldsSpace:   false,
		FieldsOrder:     []string{"component", "category", "req"},
		CustomCallerFormatter: func(f *runtime.Frame) string {
			return fmt.Sprintf(" %s:%d", f.File, f.Line)
		},
	}

	// 下面配置日志大小达到10M就会生成一个新文件，保留最近 3 天的日志文件，多余的自动清理掉。
	// 参考文章 https://blog.csdn.net/qq_42119514/article/details/121372416
	writer, _ := rotatelogs.New(
		logFilePath+"-%m%d.log",
		rotatelogs.WithLinkName(logFilePath+".log"),
		rotatelogs.WithMaxAge(time.Duration(72)*time.Hour), //保留最近 3 天的日志文件，多余的自动清理掉
		//rotatelogs.WithRotationTime(time.Duration(6)*time.Hour), // 每隔 6小时轮转一个新文件
		rotatelogs.WithRotationSize(10*1024*1024), //设置10MB大小,当大于这个容量时，创建新的日志文件
	)

	errorWriter, _ := rotatelogs.New(
		errorlogFilePath+"-%m%d.log",
		rotatelogs.WithLinkName(errorlogFilePath+".log"),
		rotatelogs.WithMaxAge(time.Duration(72)*time.Hour), //保留最近 3 天的日志文件，多余的自动清理掉
		//rotatelogs.WithRotationTime(time.Duration(6)*time.Hour), // 每隔 6小时轮转一个新文件
		rotatelogs.WithRotationSize(10*1024*1024), //设置10MB大小,当大于这个容量时，创建新的日志文件
	)
	//writers := []io.Writer{
	//	writer,
	//	errorWriter}
	////同时写文件和屏幕
	//allLevelWriter := io.MultiWriter(writers...)

	errorlfHook := lfshook.NewHook(lfshook.WriterMap{
		log.ErrorLevel: errorWriter,
		log.FatalLevel: errorWriter,
		log.PanicLevel: errorWriter,
	}, FileFormatter)
	LfHook := lfshook.NewHook(lfshook.WriterMap{
		log.DebugLevel: writer, // 为不同级别设置不同的输出目的
		log.InfoLevel:  writer,
		log.WarnLevel:  writer,
		log.ErrorLevel: writer,
		log.FatalLevel: writer,
		log.PanicLevel: writer,
	}, FileFormatter)
	log.AddHook(errorlfHook) // 输出文件
	//log.SetOutput(os.Stdout) // 输出控制台

	// 本地goland运行程序
	if strings.Contains(dir, "/tmp/GoLand") || strings.Contains(dir, "/T/GoLand") || strings.Contains(dir, "\\Temp\\GoLand") {
		log.SetLevel(log.DebugLevel)
		log.Infof("本地调试中")
		log.AddHook(LfHook)
		log.SetFormatter(StdoutFormatter)
		log.SetOutput(os.Stdout)
	} else {
		// 错误日志发送到钉钉
		// 设置日志级别
		log.SetLevel(log.InfoLevel)
		log.SetFormatter(FileFormatter)
		log.SetOutput(writer)
	}
}
