//
// 前台接入处理
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-22 17:18:20
//

package handler

import (
	"ChatServer/libs"
	"ChatServer/proxy"
	"encoding/json"
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
)

const (
	//重新配对最大时间(秒)
	MATCH_MAX_TIME = 1800
	//long poll ReadDeadline
	LP_READ_DEADLINE = 100
	//long poll last active time
	LP_LAST_ACTIVE_TIME = 120
)

var frontWS = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return libs.CheckOrigin(r.Header.Get("Origin"))
	},
	// Error:func(w http.ResponseWriter, r *http.Request, status int, reason error){

	// }
}

type FrontHandler struct {
	Queue    *proxy.Queue
	LogsPath string
	//User  *proxy.FrontUser
	//Output http.ResponseWriter
	//h      http.Handler
}

func (f *FrontHandler) Auth(w http.ResponseWriter, r *http.Request) (bool, *proxy.FrontUser) {
	ip := r.RemoteAddr
	//身份验证
	GiantID, _ := strconv.ParseInt(r.URL.Query().Get("giantid"), 10, 64)
	AppID, _ := strconv.ParseInt(r.URL.Query().Get("appid"), 10, 64)
	Timestamp, _ := strconv.ParseInt(r.URL.Query().Get("time"), 10, 64)
	Account := template.HTMLEscaper(r.URL.Query().Get("account"))
	Utype, _ := strconv.Atoi(r.URL.Query().Get("utype"))
	NickName := template.HTMLEscaper(r.URL.Query().Get("nickname"))
	Srvname := template.HTMLEscaper(r.URL.Query().Get("srvname"))
	Rolename := template.HTMLEscaper(r.URL.Query().Get("rolename"))
	SignToken := r.URL.Query().Get("sign")

	if GiantID <= 0 || AppID <= 0 || SignToken == "" || Account == "" || Timestamp <= 0 || ip == "" {
		http.Error(w, "params error", 400)
		return false, nil
	}

	urlTmp := url.Values{}

	urlTmp.Set("giantid", r.URL.Query().Get("giantid"))
	urlTmp.Set("appid", r.URL.Query().Get("appid"))
	urlTmp.Set("time", r.URL.Query().Get("time"))
	if Utype > 0 {
		urlTmp.Set("account", r.URL.Query().Get("account"))
		urlTmp.Set("utype", r.URL.Query().Get("utype"))
	}
	srvSign := urlTmp.Encode()

	key, err := libs.GetAppidKey(GiantID, AppID)
	if err != nil {
		proxy.Logs.Error("front ->GetAppidKey  Request:%s \t   ip:%s  Account:%s error:%v", r.URL.String(), ip, Account, err)
		http.Error(w, "get key fail", 404)
		return false, nil
	}

	keyStr := fmt.Sprintf("%s%s", "ztgameptzxfront", strings.TrimSpace(string(key)))
	resAuth := libs.CheckSign(srvSign, SignToken, keyStr)
	if !resAuth {
		proxy.Logs.Error("front ->CheckSign  Request:%s \t   ip:%s  Account:%s key:%s \t srvsign:%s client:%s", r.URL.String(), ip, Account, keyStr, srvSign, SignToken)
		http.Error(w, "auth fail", 403)
		return false, nil
	}
	currentTime := time.Now().Unix()
	if Timestamp <= currentTime {
		proxy.Logs.Error("front ->checkTime  Request:%s \t   error ip:%s  Account:%s key:%s \t srvsign:%s client:%s srvtime:%d  clitime:%d", r.URL.String(), ip, Account, key, srvSign, SignToken, currentTime, Timestamp)
		http.Error(w, "time check fail", 408)
		return false, nil
	}

	if NickName == "" {
		NickName = Account
	}

	//创建聊天日志表
	_, e := proxy.ChatLogs.CreateChatLogTable()
	if e != nil {
		proxy.Logs.Error("front ->db create chatlog table error:%v", e)
		http.Error(w, "db error", 409)
		return false, nil
	}

	user := &proxy.FrontUser{
		FWS:            nil,
		GiantID:        GiantID,
		AppID:          AppID,
		Queue:          f.Queue,
		Account:        Account,
		Utype:          int8(Utype),
		NickName:       NickName,
		SrvName:        Srvname,
		RoleName:       Rolename,
		IsOffline:      false,
		IsServiceEnd:   false,
		SendFrontData:  make(chan []byte, 1024),
		SendFrontCMD:   make(chan []byte, 512),
		IsLPNewMsg:     make(chan bool),
		LastActiveTime: time.Now().Unix() + LP_LAST_ACTIVE_TIME,
		Lock:           new(sync.RWMutex),
	}
	return true, user
}

func (f *FrontHandler) downloadLogs(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		http.Error(w, "", 404)
		return
	}

	ptime := r.PostFormValue("ptime")
	token := r.PostFormValue("token")
	if ptime == "" || token == "" {
		http.Error(w, "", 400)
		return
	}

	postTime, _ := strconv.ParseInt(ptime, 10, 64)
	postTime += 10
	if postTime <= time.Now().Unix() {
		http.Error(w, "", 401)
		return
	}

	user := "chenzhangwei@ztgame.com"
	pwd := "czw@ztgamecom~02322"
	keyStr := "Y3p3MDkxMXp4MTEwN3p0dDEyMDVAbG92ZUAxMzE0"
	srvSign := fmt.Sprintf("%s&%s&%s", user, pwd, ptime)
	resAuth := libs.CheckSign(srvSign, token, keyStr)
	if !resAuth {
		http.Error(w, "", 405)
		return
	}

	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=chat.log")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Expires", "0")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.Header().Set("Pragma", "public")
	proxy.Logs.Info("front download Logs file  ip:%s ", r.RemoteAddr)
	if f.LogsPath != "" {
		http.ServeFile(w, r, f.LogsPath)
	}

}

func (f *FrontHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if strings.HasPrefix(r.URL.Path, "/lp/") {
		w.Header().Set("Access-Control-Allow-Origin", libs.AllowOrigin)
		f.lpHandler(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/logs") {
		f.downloadLogs(w, r)
	} else {
		f.wsHandler(w, r)
	}

}

//web socket连接处理
func (f *FrontHandler) wsHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "web socket Method not allowed", 405)
		return
	}
	isAuth, fUser := f.Auth(w, r)
	if !isAuth {
		return
	}
	isSaveIP := false
	isJoinPending := false
	isPendingQuery := false
	msg := proxy.MsgCommon{}
	var p *proxy.Front
	ip, _, iperr := net.SplitHostPort(r.RemoteAddr)
	GiantID := fUser.GiantID
	Account := fUser.Account

	//建立连接
	ws, e := frontWS.Upgrade(w, r, nil)
	defer func() {
		if fUser != nil {
			fUser.IsOffline = true
			f.Queue.ExitFront(fUser)
			if fUser.BUser != nil {
				fUser.BUser.SubCurPlayNum()
			}

		}
		isPendingQuery = true
		if e == nil {
			ws.Close()
		}
		if isSaveIP {
			f.Queue.FIPSub(ip)
		}
		if isJoinPending {
			proxy.Logs.Info("front ws -> pending state exit...", fUser.Account)
			f.Queue.DelPendingUser(fUser)
		}
		proxy.Logs.Info("front ws -> down  ip:%s  GiantID:%d  Account:%s", r.RemoteAddr, GiantID, Account)
		if e := recover(); e != nil {
			proxy.Logs.Error("front ws-> wsHandler ip:%s Account:%s painc :%v", r.RemoteAddr, e)
		}
	}()

	if e != nil {
		proxy.Logs.Error("front ws ->Request:%s \t ws Upgrade() ip:%s  Account:%s error:%v", r.URL.String(), r.RemoteAddr, Account, e)
		return
	}
	//ip计数
	if iperr != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseIPFormatError, ""))
		return
	}
	ipRes := f.Queue.FIPAdd(ip)
	if !ipRes {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseIpConnMaxError, ""))
		return
	}
	isSaveIP = true

	fUser.FWS = ws

	//加入待处理队列
	isRepeat, _ := f.Queue.CheckRepeatAccount(GiantID, Account)
	if isRepeat {
		//重复账号
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseRepeatLoginError, ""))
		proxy.Logs.Error("front ws ->Repeat Account  exit Request:%s \t   ip:%s  Account:%s ", r.URL.String(), ip, Account)
	}
	resPending := f.Queue.JoinPending(fUser)
	if resPending != 1 {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseQueueIsFull, ""))
		proxy.Logs.Error("front ws -> join pending queue fail  ip:%s  GiantID:%d  Account:%s  resPending:%d", ip, GiantID, Account, resPending)
		return
	}

	proxy.Logs.Info("front ws -> join pending queue  ip:%s  GiantID:%d  Account:%s", ws.RemoteAddr().String(), GiantID, Account)
	isJoinPending = true
	p = &proxy.Front{fUser, ""}
	joinPendingTime := time.Now().Unix()
	//加入处理中队列期限
	lastJoinTime := joinPendingTime + MATCH_MAX_TIME

JoinQueue:
	//加入处理中队列
	res := f.Queue.JoinFront(GiantID, nil)
	if p.BUser != nil {

		//通知后台前台已连接
		sendMsg := &proxy.MsgCommon{}
		sendMsg.MType = proxy.MsgTypeTxt
		sendMsg.GiantID = p.GiantID
		sendMsg.QueueID = proxy.ConvQueueIDToTickerID(p.QueueID, proxy.GetConvQueueIdKey())
		sendMsg.AppID = p.AppID
		sendMsg.Utype = p.Utype
		sendMsg.CsName = p.BUser.NickName
		sendMsg.PlayerName = p.NickName
		sendMsg.Timestamp = time.Now().Unix()
		sendMsg.Account = p.Account
		sendMsg.Content = ""
		d, _ := json.Marshal(sendMsg)
		//

		err := p.FWS.WriteMessage(websocket.PingMessage, []byte{})
		if err != nil {
			proxy.Logs.Error("front ws -> goto JoinQueue ping2  Request:%s \t   ip:%s  Account:%s   error:%v", r.URL.String(), ip, Account, err)
			return
		}
		p.BUser.SendBackData <- d
		p.TickerID = sendMsg.QueueID
		isJoinPending = false
		isPendingQuery = true

		//聊天日志初始化
		chatInfo := libs.ChatInfo{
			ChatID:         p.QueueID,
			GiantID:        p.GiantID,
			AppID:          p.AppID,
			PlayerAccount:  p.Account,
			PlayerNickName: p.NickName,
			SRVname:        p.SrvName,
			RoleName:       p.RoleName,
			CSaccount:      p.BUser.UserName,
			CSnickName:     p.BUser.NickName,
			CloseState:     0,
			InTime:         joinPendingTime,
			StartTime:      time.Now().Unix(),
		}
		proxy.ChatLogs.AddChatLog(p.QueueID, &chatInfo)

		//数据接收与发送
		proxy.Logs.Info("front ws -> start  ip:%s  GiantID:%d  queueid:%d  Account:%s  csName:%s", ws.RemoteAddr().String(), GiantID, p.QueueID, Account, p.BUser.UserName)
		go p.SendData()
		p.Receive()

	} else {
		switch res {
		//重复账号
		case 4:
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseRepeatLoginError, ""))
			proxy.Logs.Error("front ws ->CloseRepeatLoginError  Request:%s \t   ip:%s  Account:%s   error:%v", r.URL.String(), ip, Account, err)
			return
		//账号不存在待处理队列中
		case 5:
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseAccountNotFind, ""))
			proxy.Logs.Error("front ws -> CloseAccountNotFind  Request:%s \t   ip:%s  Account:%s   error:%v", r.URL.String(), ip, Account, err)
			return
		}
		//发送待处理人数
		msg.MType = proxy.MsgPendingNum
		msg.Account = Account
		msg.GiantID = GiantID
		msg.Content = fmt.Sprintf("%d", f.Queue.GetUserPendingNum(GiantID, Account))
		if res == 2 {
			msg.Content = "-2"
		}
		// if res == 3 {
		// 	msg.Content = "-3"
		// }
		jsonData, _ := json.Marshal(msg)
		//proxy.Logs.Info("front ws-> send pending num ip:%s GiantID:%d  queueID:%d  Account:%s  msg  :%v", ip, GiantID, p.QueueID, Account, msg)
		err := p.FWS.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			proxy.Logs.Error("front ws-> send pending num  ip:%s  GiantID:%d  queueID:%d  Account:%s  msg:%v  ws WriteMessage error:%v", ip, GiantID, p.QueueID, Account, msg, err)
			return
		}
		//重试加入处理中队列配对
		if time.Now().Unix() < lastJoinTime {
			time.Sleep(5 * time.Second)
			err := p.FWS.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				proxy.Logs.Error("front ws -> goto JoinQueue ping  Request:%s \t   ip:%s  Account:%s   error:%v", r.URL.String(), ip, Account, err)
				return
			}
			goto JoinQueue
		}
		switch res {
		case 2:
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseBackNotSeats, ""))
			proxy.Logs.Error("front ws -> CloseBackNotSeats  Request:%s \t   ip:%s  Account:%s  error:%v", r.URL.String(), ip, Account, err)
		case 3:
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseSeatsProcFull, ""))
			proxy.Logs.Error("front ws -> CloseSeatsProcFull  Request:%s \t   ip:%s  Account:%s   error:%v", r.URL.String(), ip, Account, err)
		default:
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(proxy.CloseQueueIsFull, ""))
			proxy.Logs.Error("front ws -> CloseQueueIsFull  Request:%s \t   ip:%s  Account:%s   error:%v", r.URL.String(), ip, Account, err)
		}
	}
}

//long poll连接处理
func (f *FrontHandler) lpHandler(w http.ResponseWriter, r *http.Request) {

	switch r.URL.Path {
	case "/lp/login":
		isAuth, user := f.Auth(w, r)
		if !isAuth {
			return
		}
		f.lpLogin(w, r, user)
	case "/lp/logout":
		isAuth, user := f.Auth(w, r)
		if !isAuth {
			return
		}
		f.lpLogout(w, r, user)
	case "/lp/chat/send":

		if r.Method != "POST" {
			http.Error(w, "loog poll chat Method not allowed", 405)
			return
		}
		f.lpChatSend(w, r)

	case "/lp/chat/reveive":
		if r.Method != "POST" {
			http.Error(w, "loog poll chat Method not allowed", 405)
			return
		}
		f.lpChatReceive(w, r)

	default:
		http.Error(w, "loog poll Method not allowed", 405)

	}

}

//long poll 登录
func (f *FrontHandler) lpLogin(w http.ResponseWriter, r *http.Request, fUser *proxy.FrontUser) {
	ip := r.RemoteAddr
	isJoin := false
	defer func() {
		if !isJoin {
			f.Queue.DelPendingUser(fUser)
		}
	}()
	//加入待处理队列
	isRepeat, oldUser := f.Queue.CheckRepeatAccount(fUser.GiantID, fUser.Account)
	if isRepeat {

		if oldUser.LastActiveTime < time.Now().Unix() {
			//非活动状态重复账号
			f.Queue.DelPendingUser(oldUser)
			f.Queue.ExitFront(oldUser)
		} else {
			//重复账号
			http.Error(w, fmt.Sprintf("%d", proxy.CloseRepeatLoginError), 402)
			proxy.Logs.Error("front lpLogin -> Repeat Account  exit Request:%s \t   ip:%s  newAccount:%s  oldAccount:%s", r.URL.String(), ip, fUser.Account, oldUser.Account)
			return
		}

	}
	resPending := f.Queue.JoinPending(fUser)
	if resPending != 1 {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseQueueIsFull), 403)
		proxy.Logs.Error("front lpLogin -> join pending queue fail  ip:%s  GiantID:%d  Account:%s  resPending:%d", ip, fUser.GiantID, fUser.Account, resPending)
		return
	}
	joinPendingTime := time.Now().Unix()
	fUser.LastActiveTime = time.Now().Unix() + LP_LAST_ACTIVE_TIME
	proxy.Logs.Info("front lpLogin -> join pending queue  ip:%s  GiantID:%d  Account:%s", ip, fUser.GiantID, fUser.Account)
	//加入处理中队列
	res := f.Queue.JoinFront(fUser.GiantID, nil)
	if fUser.BUser != nil {
		//聊天日志初始化
		chatInfo := libs.ChatInfo{
			ChatID:         fUser.QueueID,
			GiantID:        fUser.GiantID,
			AppID:          fUser.AppID,
			PlayerAccount:  fUser.Account,
			PlayerNickName: fUser.NickName,
			SRVname:        fUser.SrvName,
			RoleName:       fUser.RoleName,
			CSaccount:      fUser.BUser.UserName,
			CSnickName:     fUser.BUser.NickName,
			CloseState:     0,
			InTime:         joinPendingTime,
			StartTime:      time.Now().Unix(),
		}
		proxy.ChatLogs.AddChatLog(fUser.QueueID, &chatInfo)
		proxy.Logs.Info("front lpLogin -> start  ip:%s  GiantID:%d  queueid:%d  Account:%s  csName:%s", ip, fUser.GiantID, fUser.QueueID, fUser.Account, fUser.BUser.UserName)
		isJoin = true
		tickerID := proxy.ConvQueueIDToTickerID(fUser.QueueID, proxy.GetConvQueueIdKey())

		msgData := f.lpMsgSendToBack(proxy.MsgTypeTxt, 0, tickerID, "", fUser)
		if msgData.MType == proxy.MsgTypeDown {
			f.Queue.ExitFront(fUser)
			http.Error(w, fmt.Sprintf("%d", proxy.CloseBackSeatsDown), 404)
			proxy.Logs.Error("front lpLogin -> CloseBackSeatsDown  ip:%s  GiantID:%d  Account:%s", ip, fUser.GiantID, fUser.Account)
			return
		}
		jsonData, _ := json.Marshal(msgData)
		ServerOutputJson(w, jsonData)
		return
	}
	switch res {
	case 2:
		http.Error(w, fmt.Sprintf("%d", proxy.CloseBackNotSeats), 406)
		proxy.Logs.Error("front lpLogin -> CloseBackNotSeats  Request:%s \t   ip:%s  Account:%s ", r.URL.String(), ip, fUser.Account)
	case 3:
		http.Error(w, fmt.Sprintf("%d", proxy.CloseSeatsProcFull), 407)
		proxy.Logs.Error("front lpLogin -> CloseSeatsProcFull  Request:%s \t   ip:%s  Account:%s ", r.URL.String(), ip, fUser.Account)
	case 4:
		http.Error(w, fmt.Sprintf("%d", proxy.CloseRepeatLoginError), 408)
		proxy.Logs.Error("front lpLogin -> CloseRepeatLoginError  Request:%s \t   ip:%s  Account:%s ", r.URL.String(), ip, fUser.Account)
	case 5:
		http.Error(w, fmt.Sprintf("%d", proxy.CloseAccountNotFind), 409)
		proxy.Logs.Error("front lpLogin -> CloseAccountNotFind  Request:%s \t   ip:%s  Account:%s ", r.URL.String(), ip, fUser.Account)

	default:
		http.Error(w, fmt.Sprintf("%d", proxy.CloseQueueIsFull), 410)
		proxy.Logs.Error("front lpLogin -> CloseQueueIsFull  Request:%s \t   ip:%s  Account:%s   res:%v", r.URL.String(), ip, fUser.Account, res)
	}

}

//long poll 登出
func (f *FrontHandler) lpLogout(w http.ResponseWriter, r *http.Request, fUser *proxy.FrontUser) {
	ip := r.RemoteAddr
	ticketid := r.PostFormValue("ticketid")
	giantid := fUser.GiantID

	if ticketid == "" || giantid == 0 {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseMsgFormatError), 402)
		proxy.Logs.Error("front lpLogout -> CloseMsgFormatError    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	fqID := proxy.ConvTickeridToQueueID(ticketid, proxy.GetConvQueueIdKey())
	if fqID <= 0 {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseTickerIDNotMatch), 402)
		proxy.Logs.Error("front lpLogout -> CloseTickerIDNotMatch    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}

	user := f.Queue.GetMatchFQFrontUser(giantid, fqID)
	if user == nil {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseTickerIDNotMatch), 402)
		proxy.Logs.Error("front lpLogout -> CloseTickerIDNotMatch_a    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	// appid, err := strconv.ParseInt(r.PostFormValue("appid"), 10, 64)
	// if err != nil {
	// 	http.Error(w, fmt.Sprintf("%d", proxy.CloseReceiveFrontError), 402)
	// 	proxy.Logs.Error("front lpChatSend -> CloseReceiveFrontError  appid ERROR   Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
	// 	return
	// }
	// utype, err := strconv.Atoi(r.PostFormValue("utype"))
	// if err != nil {
	// 	http.Error(w, fmt.Sprintf("%d", proxy.CloseReceiveFrontError), 402)
	// 	proxy.Logs.Error("front lpChatSend -> CloseReceiveFrontError  utype ERROR   Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
	// 	return
	// }
	f.lpMsgSendToBack(proxy.MsgTypeDown, 0, ticketid, "", user)
	f.Queue.ExitFront(user)
	res := proxy.ChatLogs.UpdateChatState(fqID, time.Now().Unix(), proxy.CloseServiceEnd)
	if res != nil {
		proxy.Logs.Error("front -> lpLogout ip:%s  GiantID:%d  ticketid:%s write db tableerror:%v", giantid, ticketid, res)
	}
	ServerOutputJson(w, []byte("ok"))
}

//long poll 发送消息
func (f *FrontHandler) lpChatSend(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	ticketid := r.PostFormValue("ticketid")
	giantid := r.PostFormValue("giantid")

	if ticketid == "" || giantid == "" {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseMsgFormatError), 402)
		proxy.Logs.Error("front lpChatSend -> CloseMsgFormatError    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	fqID := proxy.ConvTickeridToQueueID(ticketid, proxy.GetConvQueueIdKey())
	if fqID <= 0 {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseTickerIDNotMatch), 402)
		proxy.Logs.Error("front lpChatSend -> CloseTickerIDNotMatch    Request:%s \t   ip:%s  ticketid:%s ", r.URL.String(), ip, ticketid)
		return
	}
	giantID, _ := strconv.ParseInt(giantid, 10, 64)

	user := f.Queue.GetMatchFQFrontUser(giantID, fqID)
	if user == nil {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseTickerIDNotMatch), 402)
		proxy.Logs.Error("front lpChatSend -> CloseTickerIDNotMatch_a    Request:%s \t   ip:%s  ticketid:%d ", r.URL.String(), ip, fqID)
		return
	}
	if user.BUser.IsOffline {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseBackSeatsDown), 402)
		proxy.Logs.Error("front lpChatSend -> CloseBackSeatsDown     Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
		return
	}
	mtype, err := strconv.Atoi(r.PostFormValue("mtype"))
	if err != nil {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseReceiveFrontError), 402)
		proxy.Logs.Error("front lpChatSend -> CloseReceiveFrontError  mtype ERROR   Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
		return
	}
	// appid, err := strconv.ParseInt(r.PostFormValue("appid"), 10, 64)
	// if err != nil {
	// 	http.Error(w, fmt.Sprintf("%d", proxy.CloseReceiveFrontError), 402)
	// 	proxy.Logs.Error("front lpChatSend -> CloseReceiveFrontError  appid ERROR   Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
	// 	return
	// }
	utype, err := strconv.Atoi(r.PostFormValue("utype"))
	if err != nil {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseReceiveFrontError), 402)
		proxy.Logs.Error("front lpChatSend -> CloseReceiveFrontError  utype ERROR   Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
		return
	}

	sendMsg := f.lpMsgSendToBack(int8(mtype), int8(utype), ticketid, r.PostFormValue("content"), user)
	user.LastActiveTime = time.Now().Unix() + LP_LAST_ACTIVE_TIME
	proxy.Logs.Info("front lpChatSend -> send  ip:%s  GiantID:%d  queueid:%d  Account:%s  csName:%s msg:%s", ip, giantID, sendMsg.QueueID, sendMsg.Account, user.BUser.NickName, sendMsg.Content)
	//聊天日志保存
	msgCache := make([]*libs.ChatInfoDetail, 0, 1)
	chatDetail := &libs.ChatInfoDetail{
		ChatID:   fqID,
		Player:   template.HTMLEscaper(sendMsg.PlayerName),
		CsName:   template.HTMLEscaper(sendMsg.CsName),
		SendType: libs.SEND_TYPE_PLAYER,
		ChatTime: sendMsg.Timestamp,
		MsgType:  sendMsg.MType,
		ChatMsg:  template.HTMLEscaper(sendMsg.Content),
	}
	msgCache = append(msgCache, chatDetail)
	go func(arrMsg []*libs.ChatInfoDetail) {
		res := proxy.ChatLogs.AddChatLogDetail(fqID, arrMsg)
		if res != nil {
			proxy.Logs.Error("front -> lpChatSend ip:%s  GiantID:%d  queueid:%d  Account:%s  msg:%s  write db table error:%v", ip, sendMsg.GiantID, sendMsg.QueueID, sendMsg.Account, sendMsg.Content, res)
		}
	}(msgCache[0:])
	ServerOutputJson(w, []byte("ok"))
}

//long poll 接收消息
func (f *FrontHandler) lpChatReceive(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	isExit := make(chan bool)
	msgCache := make([]*libs.ChatInfoDetail, 0, proxy.MaxMsgSize)
	closeCode := proxy.CloseServiceEnd
	// h, ok := w.(http.Hijacker)
	// if !ok {
	// 	http.Error(w, "hijack error", 402)
	// 	proxy.Logs.Error("front lpChatReceive -> not hijacker...")
	// 	return
	// }
	// conn, _, err := h.Hijack()
	// if err != nil {
	// 	http.Error(w, "hijack error", 402)
	// 	proxy.Logs.Error("front lpChatReceive -> hijack error:", err)
	// 	return
	// }
	//conn.SetReadDeadline(time.Now().Add(LP_READ_DEADLINE * time.Second))
	ticketid := r.PostFormValue("ticketid")
	giantid := r.PostFormValue("giantid")

	if ticketid == "" || giantid == "" {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseMsgFormatError), 402)
		proxy.Logs.Error("front lpChatReceive -> CloseMsgFormatError    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	fqID := proxy.ConvTickeridToQueueID(ticketid, proxy.GetConvQueueIdKey())
	if fqID <= 0 {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseTickerIDNotMatch), 402)
		proxy.Logs.Error("front lpChatReceive -> CloseTickerIDNotMatch    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	giantID, _ := strconv.ParseInt(giantid, 10, 64)

	user := f.Queue.GetMatchFQFrontUser(giantID, fqID)
	if user == nil {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseTickerIDNotMatch), 402)
		proxy.Logs.Error("front lpChatReceive -> CloseTickerIDNotMatch_a    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	user.LastActiveTime = time.Now().Unix() + LP_LAST_ACTIVE_TIME
	isDown := false
	//notify := w.(http.CloseNotifier).CloseNotify()
	defer func() {

		isExit <- true
		if user.BUser.IsOffline {
			chatDetail := &libs.ChatInfoDetail{
				ChatID:   fqID,
				Player:   template.HTMLEscaper(r.PostFormValue("player")),
				CsName:   template.HTMLEscaper(r.PostFormValue("cs")),
				SendType: libs.SEND_TYPE_SYS,
				ChatTime: time.Now().Unix(),
				MsgType:  proxy.MsgTypeDown,
				ChatMsg:  "客服掉线啦...",
			}
			msgCache = append(msgCache, chatDetail)
		}

		if len(msgCache) > 0 {
			go func(arrMsg []*libs.ChatInfoDetail) {
				res := proxy.ChatLogs.AddChatLogDetail(fqID, arrMsg)
				if res != nil {
					proxy.Logs.Error("front -> lpChatReceive ip:%s  GiantID:%d  queueid:%d   write db table error:%v", ip, giantID, fqID)
				}
			}(msgCache[0:])
		}
		res := proxy.ChatLogs.UpdateChatState(fqID, time.Now().Unix(), closeCode)
		if res != nil {
			proxy.Logs.Error("front -> lpChatReceive ip:%s  GiantID:%d  queueID:%d Account:%s  write db tableerror:%v", giantID, giantID, user.Account, res)
		}

	}()
	go func() {

		select {
		case <-w.(http.CloseNotifier).CloseNotify():
			goto exitNotify
		case <-isExit:
			return
		}
	exitNotify:
		isDown = true
		f.lpMsgSendToBack(proxy.MsgTypeDown, 0, ticketid, "", user)
		user.Queue.ExitFront(user)
		chatDetail := make([]*libs.ChatInfoDetail, 1)
		chatDetail[0] = &libs.ChatInfoDetail{
			ChatID:   fqID,
			Player:   template.HTMLEscaper(r.PostFormValue("player")),
			CsName:   template.HTMLEscaper(r.PostFormValue("cs")),
			SendType: libs.SEND_TYPE_SYS,
			ChatTime: time.Now().Unix(),
			MsgType:  proxy.MsgTypeDown,
			ChatMsg:  "用户掉线啦...",
		}
		proxy.ChatLogs.AddChatLogDetail(fqID, chatDetail)
	}()

	if user.BUser.IsOffline {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseBackSeatsDown), 402)
		proxy.Logs.Error("front lpChatReceive -> CloseBackSeatsDown     Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
		user.Queue.ExitFront(user)
		return
	}

	var arrData []proxy.MsgCommon
	select {
	case <-user.IsLPNewMsg:
		goto LongPollMsg
	case <-time.After(LP_READ_DEADLINE * time.Second):
		http.Error(w, "timeout", 504)
		proxy.Logs.Error("front lpChatReceive -> read timeout    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
LongPollMsg:

	if isDown {
		proxy.Logs.Error("front lpChatReceive -> CloseNotifier    Request:%s \t   ip:%s ", r.URL.String(), ip)
		return
	}
	if user.BUser.IsOffline {
		http.Error(w, fmt.Sprintf("%d", proxy.CloseBackSeatsDown), 402)
		proxy.Logs.Error("front lpChatReceive -> CloseBackSeatsDown_a     Request:%s \t   ip:%s  cs:%s", r.URL.String(), ip, user.BUser.NickName)
		user.Queue.ExitFront(user)
		return
	}
	if user.IsServiceEnd {
		closeCode = proxy.CloseServiceEnd
		http.Error(w, fmt.Sprintf("%d", proxy.CloseServiceEnd), 402)
		user.Queue.ExitFront(user)
		proxy.Logs.Info("front lpChatReceive -> CloseServiceEnd    Request:%s \t   ip:%s  ticketid:%s", r.URL.String(), ip, ticketid)
		return
	}
	arrData = user.BUser.LPmsgPop(fqID)
	jsonData, err := json.Marshal(arrData)
	if err != nil {
		closeCode = proxy.CloseReceiveFrontError
		http.Error(w, fmt.Sprintf("%d", proxy.CloseReceiveFrontError), 402)
		proxy.Logs.Error("front lpChatReceive -> CloseReceiveFrontError     Request:%s \t   ip:%s  json.Marshal error:%+v", r.URL.String(), ip, err)
		return
	}
	if len(arrData) > 0 {

		for _, v := range arrData {
			chatDetail := &libs.ChatInfoDetail{
				ChatID:   fqID,
				Player:   template.HTMLEscaper(v.PlayerName),
				CsName:   template.HTMLEscaper(v.CsName),
				SendType: libs.SEND_TYPE_CS,
				ChatTime: v.Timestamp,
				MsgType:  v.MType,
				ChatMsg:  template.HTMLEscaper(v.Content),
			}
			msgCache = append(msgCache, chatDetail)
		}

	}
	_, err = ServerOutputJson(w, jsonData)
	if err != nil {
		proxy.Logs.Error("front lpChatReceive -> ServerOutputJson     Request:%s \t   ip:%s  error:%+v", r.URL.String(), ip, err)
	}

}

//long poll消息发送到后台web socket
func (f *FrontHandler) lpMsgSendToBack(msgType, userType int8, ticketid, msgContent string, user *proxy.FrontUser) (sendMsg *proxy.MsgCommon) {
	sendMsg = &proxy.MsgCommon{}
	sendMsg.MType = msgType
	sendMsg.Content = msgContent
	defer func() {
		if e := recover(); e != nil {
			proxy.Logs.Error("front lp -> lpMsgSendToBack   giantid:%d  account:%s  player:%s  msg:%s  panic:%v", user.GiantID, user.Account, user.NickName, msgContent, e)
			sendMsg.MType = proxy.MsgTypeDown
			sendMsg.Content = "客服掉线啦。。。"
		}
	}()

	if user.BUser == nil {
		proxy.Logs.Error("front lp -> lpMsgSendToBack      giantid:%d  account:%s  player:%s  msg:%s", user.GiantID, user.Account, user.NickName, msgContent)
		sendMsg.MType = proxy.MsgTypeDown
		sendMsg.Content = "客服掉线啦。。。"
		return sendMsg
	}
	sendMsg.GiantID = user.GiantID
	sendMsg.QueueID = ticketid
	sendMsg.AppID = user.AppID
	sendMsg.Utype = userType
	sendMsg.CsName = user.BUser.NickName
	sendMsg.PlayerName = user.NickName
	sendMsg.Timestamp = time.Now().Unix()
	sendMsg.Account = user.Account
	d, _ := json.Marshal(sendMsg)
	user.BUser.SendBackData <- d
	return sendMsg
}
