//
// 后台接入处理
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-22 18:07:16
//

package handler

import (
	"ChatServer/libs"
	"ChatServer/proxy"
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	//"runtime/pprof"
	//"os"
	"container/list"
)

var backtWS = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return libs.CheckOrigin(r.Header.Get("Origin"))
	},
}

type BackHandler struct {
	Queue *proxy.Queue
}

func (b *BackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	isJoin := false
	isSaveIP := false
	ip, _, iperr := net.SplitHostPort(r.RemoteAddr)
	// goroutine:=pprof.Lookup("goroutine")
	// goroutine.WriteTo(os.Stdout,2)
	// heap:=pprof.Lookup("heap")
	// heap.WriteTo(os.Stdout,2)
	//身份验证

	GiantID, _ := strconv.ParseInt(r.URL.Query().Get("giantid"), 10, 64)
	AppID, _ := strconv.ParseInt(r.URL.Query().Get("appid"), 10, 64)
	Timestamp, _ := strconv.ParseInt(r.URL.Query().Get("time"), 10, 64)
	Account := template.HTMLEscaper(r.URL.Query().Get("account"))
	Nickname := template.HTMLEscaper(r.URL.Query().Get("nickname"))
	SignToken := template.HTMLEscaper(r.URL.Query().Get("sign"))

	if GiantID <= 0 || AppID <= 0 || SignToken == "" || Account == "" || Timestamp <= 0 {
		http.Error(w, "params error", 400)
		return
	}

	urlTmp := url.Values{}
	urlTmp.Set("giantid", r.URL.Query().Get("giantid"))
	urlTmp.Set("appid", r.URL.Query().Get("appid"))
	urlTmp.Set("time", r.URL.Query().Get("time"))
	srvSign := urlTmp.Encode()

	key, err := libs.GetAppidKey(GiantID, AppID)
	if err != nil {
		proxy.Logs.Error("back ->GetAppidKey  Request:%s \t   ip:%s  Nickname:%s error:%v", r.URL.String(), ip, Nickname, err)
		http.Error(w, "get key fail", 404)
		return
	}

	keyStr := fmt.Sprintf("%s%s", "ztgameptzxback", strings.TrimSpace(string(key)))
	resAuth := libs.CheckSign(srvSign, SignToken, keyStr)
	if !resAuth {
		proxy.Logs.Error("back ->CheckSign  Request:%s \t   ip:%s  Nickname:%s key:%s \t srvsign:%s client:%s", r.URL.String(), ip, Nickname, keyStr, srvSign, SignToken)
		http.Error(w, "auth fail", 403)
		return
	}
	currentTime := time.Now().Unix()
	if Timestamp <= currentTime {
		proxy.Logs.Error("back ->checkTime  Request:%s \t   error ip:%s  Nickname:%s key:%s \t srvsign:%s client:%s srvtime:%d clitime:%d", r.URL.String(), ip, Nickname, key, srvSign, SignToken, currentTime, Timestamp)
		http.Error(w, "time check fail", 408)
		return
	}

	if Nickname == "" {
		Nickname = Account
	}

	//创建聊天日志表
	_, e := proxy.ChatLogs.CreateChatLogTable()
	if e != nil {
		proxy.Logs.Error("back ->db create chatlog table error:%v", e)
		http.Error(w, "db error", 500)
		return
	}

	//建立连接
	ws, err := backtWS.Upgrade(w, r, nil)
	var user *proxy.BackUser
	defer func() {
		if isJoin {
			user.IsOffline = true
			b.Queue.ExitBack(user)
		}
		if err == nil {
			ws.Close()
		}
		if isSaveIP {
			b.Queue.BIPSub(ip)

		}
		//close(user.Offline)
		proxy.Logs.Info("back -> down  ip:%s  GiantID:%d  cs:%s  err:%v", r.RemoteAddr, GiantID, Nickname, err)
	}()
	if err != nil {
		proxy.Logs.Error("back -> Request:%s \t ws Upgrade()  cs:%s error:%v", r.URL.String(), Nickname, err)
		return
	}

	//加入后台队列
	user = &proxy.BackUser{
		BWS:           ws,
		GiantID:       GiantID,
		Queue:         b.Queue,
		UserName:      Account,
		NickName:      Nickname,
		SendFrontData: make(chan []byte, 1024),
		SendBackData:  make(chan []byte, 1024),
		SendBackCMD:   make(chan []byte, 512),
		AppID:         AppID,
		IsOffline:     false,
		Lock:          new(sync.RWMutex),
		LockNum:       new(sync.RWMutex),
		CurPlayerNum:  0,
		LockMatch:     new(sync.RWMutex),
		LPmsg:         make(map[int64]*list.List, proxy.MaxDevCSNum),
		LPLock:        new(sync.RWMutex),
	}
	p := &proxy.Back{user}
	//重复账号检查
	isRepeat, _ := b.Queue.CheckBackQRepeatAccount(GiantID, Account)
	if isRepeat {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseRepeatLoginError, ""))
		proxy.Logs.Error("back ->Repeat Account  exit  Request:%s \t   ip:%s  Account:%s  ", r.URL.String(), ip, Account)
		return
	}
	//ip计数
	if iperr != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseIPFormatError, ""))
		return
	}
	ipRes := b.Queue.BIPAdd(ip)
	if !ipRes {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseIpConnMaxError, ""))
		return
	}
	isSaveIP = true
	res := b.Queue.JoinBack(user)
	if res {
		isJoin = true
		//
		proxy.Logs.Info("back -> join  ip:%s  GiantID:%d  QueueID:%d  cs:%s", ws.RemoteAddr().String(), GiantID, user.QueueID, Nickname)
		go p.SendFrontPump()
		go p.SendBackPump()
		p.ReceiveBack()

	} else {
		err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseQueueIsFull, ""))
		proxy.Logs.Error("back -> CloseQueueIsFull:%s \t ws Upgrade()  cs:%s error:%v", r.URL.String(), Nickname, err)
	}

}
