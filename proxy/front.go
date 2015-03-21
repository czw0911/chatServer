//
// 前台网关
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-21 16:42:24
//

package proxy

import (
	"ChatServer/libs"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"strings"
	"text/template"
	"time"
)

type Front struct {
	*FrontUser
	TickerID string
}

//接收消息后转发到后台
func (f *Front) Receive() {
	ip := f.FWS.RemoteAddr().String()
	closeCode := 0
	msgCache := make([]*libs.ChatInfoDetail, 0, 20)
	defer func() {
		if f.QueueID > 0 && f.BUser.IsOffline {
			closeCode = CloseBackSeatsDown
		}
		if len(msgCache) > 0 && f.QueueID > 0 {

			res := ChatLogs.AddChatLogDetail(f.QueueID, msgCache)
			if res != nil {
				Logs.Error("front -> Recevie defer func ip:%s  GiantID:%d  queueid:%d  Account:%s   write db table error:%v", ip, f.GiantID, f.QueueID, f.Account, res)
			}
			msgCache = nil
		}
		f.sendDownToBack(closeCode)
		if e := recover(); e != nil {
			Logs.Error("front -> Recevie ip:%s Account:%s painc :%v", ip, f.Account, e)
		}
		Logs.Error("front -> Recevie exit ip:%s Account:%s close code:%d", ip, f.Account, closeCode)
	}()

	endMsgTime := time.Now().Unix()
	f.FWS.SetReadLimit(maxMessageSize)
	f.FWS.SetReadDeadline(time.Now().Add(pongWait))
	f.FWS.SetPongHandler(func(string) error {
		sumTIime := time.Now().Unix() - endMsgTime
		//空闲超过10分钟
		if sumTIime > 600 {
			_ = f.send(websocket.CloseMessage, websocket.FormatCloseMessage(CloseLinkIdleError, ""))
		} else {
			f.FWS.SetReadDeadline(time.Now().Add(pongWait))
		}
		return nil
	})

	var msg MsgCommon
	var jsonData []byte

	//f.TickerID = ConvQueueIDToTickerID(f.QueueID, queueIdKey)

	for {

		if f.IsServiceEnd {
			closeCode = CloseServiceEnd
			return
		}

		if f.QueueID > 0 && f.BUser.IsOffline {
			closeCode = CloseBackSeatsDown
			return
		}

		if f.IsOffline {
			closeCode = CloseFrontDown
			return
		}

		mt, message, err := f.FWS.ReadMessage()
		if err != nil {
			Logs.Error("front -> recevie ip:%s  GiantID:%d  queueID:%d  Account:%s  error:%v", ip, f.GiantID, f.QueueID, f.Account, err)
			closeCode = CloseReceiveFrontError
			return
		}

		if mt == websocket.CloseMessage {
			closeCode = CloseFrontDown
			return
		}
		if mt == websocket.TextMessage {

			e := json.Unmarshal(message, &msg)
			if e != nil {
				//消息格式错误
				Logs.Error("front -> recevie ip:%s  GiantID:%d  queueID:%d  Account:%s  msg:%s  format error:%v", ip, f.GiantID, f.QueueID, f.Account, message, e)
				closeCode = CloseMsgFormatError
				return
			}
			if msg.MType == MsgTypeDown {
				closeCode = CloseFrontDown
				return
			}
			if msg.MType == MsgPendingNum {
				msg.Content = fmt.Sprintf("%d", f.Queue.GetPendingLen(f.GiantID))
				jsonData, _ = json.Marshal(msg)
				//Logs.Info("front ->recevie  MsgPendingNum ip:%s Account:%s  msg :%v", ip, f.Account, msg)
				f.SendFrontCMD <- jsonData
				continue
			}
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			endMsgTime = time.Now().Unix()
			msg.QueueID = f.TickerID
			msg.PlayerName = f.NickName
			msg.CsName = f.BUser.NickName
			msg.Timestamp = endMsgTime
			d, _ := json.Marshal(msg)
			//转发后台
			f.BUser.SendBackData <- d
			Logs.Info("front -> recevie ip:%s  GiantID:%d  queueid:%d  Account:%s  msg:%s", ip, f.GiantID, f.QueueID, f.Account, message)
			//聊天日志保存
			chatDetail := &libs.ChatInfoDetail{
				ChatID:   f.QueueID,
				Player:   template.HTMLEscaper(msg.PlayerName),
				CsName:   template.HTMLEscaper(msg.CsName),
				SendType: libs.SEND_TYPE_PLAYER,
				ChatTime: endMsgTime,
				MsgType:  msg.MType,
				ChatMsg:  template.HTMLEscaper(msg.Content),
			}
			msgCache = append(msgCache, chatDetail)
			if len(msgCache) > 10 {
				go func(arrMsg []*libs.ChatInfoDetail) {
					res := ChatLogs.AddChatLogDetail(f.QueueID, arrMsg)
					if res != nil {
						Logs.Error("front -> recevie ip:%s  GiantID:%d  queueid:%d  Account:%s  msg:%s  write db table error:%v", ip, f.GiantID, f.QueueID, f.Account, message, res)
					}
				}(msgCache[:10])
				msgCache = msgCache[10:]
			}
		}
		//time.Sleep(forWaitTime)
	}
}

//发送掉线消息给后台
func (f *Front) sendDownToBack(closeCode int) {
	ip := f.FWS.RemoteAddr().String()
	f.Lock.Lock()
	defer func() {
		if e := recover(); e != nil {
			Logs.Error("front -> sendDownToBack ip:%s Account:%s painc :%v", ip, e)
		}

		f.Lock.Unlock()
	}()
	if f.IsOffline {
		return
	}
	//
	res := ChatLogs.UpdateChatState(f.QueueID, time.Now().Unix(), closeCode)
	if res != nil {
		Logs.Error("front -> sendDownToBack ip:%s  GiantID:%d  queueID:%d Account:%s  write db tableerror:%v", ip, f.GiantID, f.QueueID, f.Account, res)
	}
	//
	if CloseServiceEnd != closeCode {
		msg := "掉线啦..."
		switch closeCode {
		case CloseBackNotSeats:
			fallthrough
		case CloseBackSeatsDown:
			msg = "客服掉线啦..."
		case CloseFrontDown:
			fallthrough
		case CloseReceiveFrontError:
			msg = "用户掉线啦..."
		}
		chatDetail := make([]*libs.ChatInfoDetail, 1)
		chatDetail[0] = &libs.ChatInfoDetail{
			ChatID:   f.QueueID,
			Player:   template.HTMLEscaper(f.NickName),
			CsName:   template.HTMLEscaper(f.BUser.NickName),
			SendType: libs.SEND_TYPE_SYS,
			ChatTime: time.Now().Unix(),
			MsgType:  MsgTypeDown,
			ChatMsg:  msg,
		}
		res := ChatLogs.AddChatLogDetail(f.QueueID, chatDetail)
		if res != nil {
			Logs.Error("front -> sendDownToBack ip:%s  GiantID:%d  queueid:%d  Account:%s    write db table error:%v", ip, f.GiantID, f.QueueID, f.Account, res)
		}
	}

	//
	f.IsOffline = true
	//f.BUser.SubCurPlayNum()

	_ = f.send(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, ""))

	m := &MsgCommon{MType: MsgTypeDown, GiantID: f.GiantID, AppID: f.AppID, QueueID: f.TickerID, Account: "", PlayerName: f.NickName, CsName: f.BUser.NickName}
	d, err := json.Marshal(m)
	if err != nil {
		Logs.Error("front -> sendDownToBack ip:%s  GiantID:%d  queueID:%d Account:%s  json marsha1 error:%v", ip, f.GiantID, f.QueueID, f.Account, err)
	}
	if !f.BUser.IsOffline {
		f.BUser.SendBackCMD <- d
	}
	f.FWS.Close()
	f.Queue.ExitFront(f.FrontUser)
	close(f.SendFrontData)
}

func (f *Front) send(mt int, msg []byte) error {
	f.FWS.SetWriteDeadline(time.Now().Add(writeWait))
	return f.FWS.WriteMessage(mt, msg)
}

//发送消息到前台
func (f *Front) SendData() {
	ip := f.FWS.RemoteAddr().String()
	closeCode := CloseRespondCheck
	ticker := time.NewTicker(pingPeriod)
	msgCache := make([]*libs.ChatInfoDetail, 0, 20)
	defer func() {
		ticker.Stop()
		if len(msgCache) > 0 {

			res := ChatLogs.AddChatLogDetail(f.QueueID, msgCache)
			if res != nil {
				Logs.Error("front -> SendData defer func ip:%s  GiantID:%d  queueid:%d  Account:%s   write db table error:%v", ip, f.GiantID, f.QueueID, f.Account, res)
			}
			msgCache = nil
		}
		f.sendDownToBack(closeCode)
		if e := recover(); e != nil {
			Logs.Error("front -> SendData ip:%s Account:%s painc :%v", ip, f.Account, e)
		}
	}()
	var msg MsgCommon
	for {

		if f.IsServiceEnd {
			closeCode = CloseServiceEnd
			return
		}

		if f.BUser.IsOffline {
			closeCode = CloseBackSeatsDown
			return
		}
		if f.IsOffline {
			closeCode = CloseFrontDown
			return
		}

		select {
		case message, ok := <-f.SendFrontData:
			if !ok {
				Logs.Error("front -> SendData ip:%s  GiantID:%d  queueid:%d Account:%s  <-f.SendFrontData  error:%s", ip, f.GiantID, f.QueueID, f.Account, ok)
				closeCode = CloseReceiveBackError
				return
			}
			if err := f.send(websocket.TextMessage, message); err != nil {
				Logs.Error("front -> SendData ip:%s  GiantID:%d  queueid:%d Account:%s send data  error:%s", ip, f.GiantID, f.QueueID, f.Account, err)
				closeCode = CloseSendToFrontErr
				return
			}
			e := json.Unmarshal(message, &msg)
			if e != nil {
				//消息格式错误
				Logs.Error("front -> SendData ip:%s  GiantID:%d  queueID:%d  Account:%s  msg:%s  format error:%v", ip, f.GiantID, f.QueueID, f.Account, message, e)
				closeCode = CloseMsgFormatError
				return
			}
			//聊天日志保存
			chatDetail := &libs.ChatInfoDetail{
				ChatID:   f.QueueID,
				Player:   template.HTMLEscaper(msg.PlayerName),
				CsName:   template.HTMLEscaper(msg.CsName),
				SendType: libs.SEND_TYPE_CS,
				ChatTime: msg.Timestamp,
				MsgType:  msg.MType,
				ChatMsg:  template.HTMLEscaper(msg.Content),
			}
			msgCache = append(msgCache, chatDetail)
			if len(msgCache) > 10 {
				go func(arrMsg []*libs.ChatInfoDetail) {
					res := ChatLogs.AddChatLogDetail(f.QueueID, arrMsg)
					if res != nil {
						Logs.Error("front -> SendData ip:%s  GiantID:%d  queueid:%d  Account:%s  msg:%s  write db table error:%v", ip, f.GiantID, f.QueueID, f.Account, message, res)
					}
				}(msgCache[:10])
				msgCache = msgCache[10:]
			}

		case msgCMD, ok := <-f.SendFrontCMD:
			if !ok {
				Logs.Error("front -> SendFrontCMD ip:%s  GiantID:%d  queueid:%d Account:%s  <-f.SendFrontCMD  error:%s", ip, f.GiantID, f.QueueID, f.Account, ok)
				closeCode = CloseReceiveBackError
				return
			}
			if err := f.send(websocket.TextMessage, msgCMD); err != nil {
				Logs.Error("front -> SendFrontCMD ip:%s  GiantID:%d  queueid:%d Account:%s send data  error:%s", ip, f.GiantID, f.QueueID, f.Account, err)
				closeCode = CloseSendToFrontErr
				return
			}
		case <-ticker.C:
			//心跳检测
			if err := f.send(websocket.PingMessage, []byte{}); err != nil {
				Logs.Error("front -> SendData ip:%s  GiantID:%d  queueid:%d  Account:%s ping data  error:%s", ip, f.GiantID, f.QueueID, f.Account, err)
				closeCode = CloseRespondCheck
				return
			}
		}

		//time.Sleep(forWaitTime)
	}
}
