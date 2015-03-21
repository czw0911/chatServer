//
// web socket 聊天服务消息转发系统
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//
package main

import (
	"ChatServer/handler"
	"ChatServer/libs"
	"ChatServer/proxy"
	"ChatServer/server"
	"fmt"
	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"path/filepath"
	"runtime"
	//"runtime/pprof"
)

const VERSION = "1.2.1"

var (
	Conf      config.ConfigContainer
	SrvName   string
	LogsPath  string
	BackPort  int
	FrontPort int
)

func main() {

	proxy.Logs.Info("start %s", SrvName)
	//设置队列
	queue := proxy.NewQueue()

	//前端
	fh := handler.FrontHandler{Queue: queue, LogsPath: LogsPath}
	fs := &server.Server{
		Port:    FrontPort,
		Handler: &fh,
	}
	exit := make(chan bool, 1)
	go func() {
		proxy.Logs.Info("front Running on %d", FrontPort)
		err := fs.Run()
		if err != nil {
			proxy.Logs.Info("front Running err:", err)
			exit <- true
		}

	}()

	//后端
	bh := handler.BackHandler{queue}
	bs := &server.Server{
		Port:    BackPort,
		Handler: &bh,
	}
	go func() {
		proxy.Logs.Info("back Running on %d", BackPort)
		err := bs.Run()
		if err != nil {
			proxy.Logs.Info("back Running err:", err)
			exit <- true
		}
	}()

	// p:=pprof.Lookup("goroutine")
	// p.WriteTo(os.Stdout,2)

	<-exit

}

func init() {
	// 初始化配置文件
	workPath, _ := os.Getwd()
	workPath, _ = filepath.Abs(workPath)
	configFile := filepath.Join(workPath, "chatserver.conf")
	if _, err := os.Stat(configFile); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("init config file path error:", err)
			os.Exit(0)
		}
	}
	Conf, err := config.NewConfig("ini", configFile)
	if err != nil {
		fmt.Println("init config file error:", err)
		os.Exit(0)
	}

	//初始化服务器端口
	srvName := Conf.String("appName")
	if srvName == "" {
		srvName = "chatServer"
	}
	SrvName = fmt.Sprintf("%s_%s", srvName, VERSION)

	BackPort, err = Conf.Int("backPort")
	if err != nil {
		fmt.Println("init backPort error:", err)
		os.Exit(0)
	}

	FrontPort, err = Conf.Int("frontPort")
	if err != nil {
		fmt.Println("init frontPort error:", err)
		os.Exit(0)
	}

	proxy.MaxDevNum, err = Conf.Int("MaxDevNum")
	if err != nil {
		fmt.Println("init MaxDevNum error:", err)
		os.Exit(0)
	}

	proxy.MaxDevPendingNum, err = Conf.Int("MaxDevPendingNum")
	if err != nil {
		fmt.Println("init MaxDevPendingNum error:", err)
		os.Exit(0)
	}

	proxy.MaxDevCSNum, err = Conf.Int("MaxDevCSNum")
	if err != nil {
		fmt.Println("init MaxDevCSNum error:", err)
		os.Exit(0)
	}

	proxy.MaxCSProcNum, err = Conf.Int("MaxCSProcNum")
	if err != nil {
		fmt.Println("init MaxCSProcNum error:", err)
		os.Exit(0)
	}

	proxy.MaxIPconn, err = Conf.Int("MaxIPconn")
	if err != nil {
		fmt.Println("init MaxIPconn error:", err)
		os.Exit(0)
	}

	libs.GetKeyServerUrl = Conf.String("KeyServiceUrl")
	libs.AllowOrigin = Conf.String("AllowDoaminOrIP")

	//初始化日志
	proxy.Logs = logs.NewLogger(10000)
	logFile := Conf.String("log_file_path")
	LogsPath = logFile
	logOutType := "file"
	if logFile == "" {
		logOutType = "console"
	} else {
		logFile = `{"filename":"` + logFile + `"}`
	}
	//println(logOutType, logFile)
	err = proxy.Logs.SetLogger(logOutType, logFile)
	if err != nil {
		fmt.Println("init console log error:", err)
	}

	// database
	dbUser := Conf.String("mysql_db_user")
	dbPass := Conf.String("mysql_db_pass")
	dbHost := Conf.String("mysql_db_host")
	dbPort := Conf.String("mysql_db_port")
	dbName := Conf.String("mysql_db_name")
	dbCharset := Conf.String("mysql_db_charset")
	maxIdleConn, _ := Conf.Int("mysql_db_max_idle_conn")
	maxOpenConn, _ := Conf.Int("mysql_db_max_open_conn")
	dbLink := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s", dbUser, dbPass, dbHost, dbPort, dbName, dbCharset) + "&loc=Asia%2FShanghai"
	//orm.Debug = true
	orm.RegisterDriver("mysql", orm.DR_MySQL)
	orm.RegisterDataBase("default", "mysql", dbLink, maxIdleConn, maxOpenConn)

	proxy.ChatLogs = libs.NewChatLog()

	runtime.GOMAXPROCS(runtime.NumCPU())
}
