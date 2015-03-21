//
// 后台网关
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-21 17:36:58
//

package proxy

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

type Back struct {
	*BackUser
}

//接收后台消息
func (b *Back) ReceiveBack() {
	ip := b.BWS.RemoteAddr().String()
	closeCode := CloseRespondCheck
	defer func() {
		b.sendDownMsgToFront(closeCode)
		if e := recover(); e != nil {
			Logs.Error("back ->  ReceiveBack ip:%s  cs:%s  painc:%v", ip, b.UserName, e)
		}
		Logs.Error("back ->  ReceiveBack exit ip:%s  cs:%s  close code:%d", ip, b.UserName, closeCode)
	}()
	b.BWS.SetReadLimit(maxMessageSize)
	b.BWS.SetReadDeadline(time.Now().Add(pongWaitBack))
	b.BWS.SetPongHandler(func(string) error { b.BWS.SetReadDeadline(time.Now().Add(pongWaitBack)); return nil })

	for {

		if b.IsOffline {
			closeCode = CloseBackDown
			return
		}

		mt, message, err := b.BWS.ReadMessage()
		if err != nil {
			closeCode = CloseReveiceBackWS
			Logs.Error("back-> ReceiveBack ip:%s GiantID:%d  QueueID:%d   cs:%s mt:%d  error:%v", ip, b.GiantID, b.QueueID, b.UserName, mt, err)
			return
		}
		if mt == websocket.CloseMessage {
			closeCode = CloseBackDown
			return
		}
		if mt == websocket.TextMessage {
			b.SendFrontData <- message
			Logs.Info("back -> ReceiveBack ip:%s  GiantID:%d  QueueID:%d  cs:%s   msg:%s", ip, b.GiantID, b.QueueID, b.UserName, message)
		}

		//time.Sleep(forWaitTime)
	}
}

//消息发送到台台
func (b *Back) SendBackPump() {
	ticker := time.NewTicker(pingPeriodBack)
	closeCode := CloseBackSeatsDown
	defer func() {
		ticker.Stop()
		b.sendDownMsgToFront(closeCode)
		if e := recover(); e != nil {
			Logs.Error("back ->  SendBackPump_c ip%s  cs:%s  painc:%v", b.BWS.RemoteAddr().String(), b.UserName, e)
		}
		Logs.Error("back ->  SendBackPump exit ip:%s  cs:%s  close code:%d", b.BWS.RemoteAddr().String(), b.UserName, closeCode)
	}()
	for {
		if b.IsOffline {
			closeCode = CloseBackDown
			return
		}
		select {
		case message, ok := <-b.SendBackData:
			if !ok {
				closeCode = CloseReveiceBackChan
				Logs.Error("back ->  SendBackPump_a ip%s  cs:%s  err:%v", b.BWS.RemoteAddr().String(), b.UserName, ok)
				return
			}

			err := b.sendBack(websocket.TextMessage, message)
			if err != nil {
				closeCode = CloseFrontToBackError
				Logs.Error("back ->  SendBackPump_b ip%s  cs:%s  err:%v", b.BWS.RemoteAddr().String(), b.UserName, err)
				return
			}
		case cmdMsg, ok := <-b.SendBackCMD:
			if !ok {
				closeCode = CloseReveiceBackCMD
				Logs.Error("back ->  SendBackPump_c ip%s  cs:%s  err:%v", b.BWS.RemoteAddr().String(), b.UserName, ok)
				return
			}
			err := b.sendBack(websocket.TextMessage, cmdMsg)
			if err != nil {
				closeCode = CloseFrontToBackError
				Logs.Error("back ->  SendBackPump_d ip%s  cs:%s  err:%v", b.BWS.RemoteAddr().String(), b.UserName, err)
				return
			}

		case <-ticker.C:
			//心跳检测
			if err := b.sendBack(websocket.PingMessage, []byte{}); err != nil {
				closeCode = CloseRespondCheck
				return
			}
		}

		//time.Sleep(forWaitTime)
	}
}

func (b *Back) sendBack(mt int, msg []byte) error {
	b.BWS.SetWriteDeadline(time.Now().Add(writeWait))
	return b.BWS.WriteMessage(mt, msg)
}

//后台掉线
func (b *Back) sendDownMsgToFront(closeCode int) {
	b.Lock.Lock()
	ip := b.BWS.RemoteAddr().String()
	defer func() {
		if e := recover(); e != nil {
			Logs.Error("back -> sendDownMsgToFront ip:%s  cs:%s  painc :%v", ip, b.UserName, e)
		}
		b.Lock.Unlock()
	}()
	if b.IsOffline {
		return
	}
	b.IsOffline = true
	_ = b.sendBack(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, ""))
	_ = b.BWS.Close()
	b.Queue.ExitBack(b.BackUser)

	Logs.Error("back-> sendDownMsgToFront ip:%s GiantID:%d  QueueID:%d   cs:%s closeCode:%d  offline...", ip, b.GiantID, b.QueueID, b.UserName, closeCode)
	close(b.SendBackData)
	close(b.SendFrontData)
	close(b.SendBackCMD)
	//time.Sleep(30 * time.Second)
}

//发送到前台
func (b *Back) SendFrontPump() {
	ip := b.BWS.RemoteAddr().String()
	closeCode := 0
	//pendingNumMsg := new(MsgCommon)
	var msg MsgCommon
	var jsonData []byte
	var res int8
	defer func() {
		b.sendDownMsgToFront(closeCode)
		if e := recover(); e != nil {
			Logs.Error("back -> SendFrontPump  ip:%s   cs:%s  painc :%v", ip, b.UserName, e)
		}
		Logs.Error("back ->  SendFrontPump exit ip:%s  cs:%s  close code:%d", ip, b.UserName, closeCode)
	}()
	for {
		if b.IsOffline {
			closeCode = CloseBackDown
			return
		}
		select {
		case message, ok := <-b.SendFrontData:

			if !ok {
				closeCode = CloseReveiceBackChan
				return
			}
			err := json.Unmarshal(message, &msg)
			if err != nil {
				closeCode = CloseMsgFormatError
				return
			}
			Logs.Info("back ->SendFrontPump msg:%#v", msg)
			switch msg.MType {
			case MsgTypeDown:
				closeCode = CloseBackDown
				return
			case MsgPendingNum:
				msg.Content = fmt.Sprintf("%d", b.Queue.GetPendingLen(b.GiantID))
				jsonData, _ = json.Marshal(msg)
				Logs.Info("back ->SendFrontPump  MsgPendingNum ip:%s Account:%s msg  :%v", ip, b.UserName, msg)
				b.SendBackCMD <- jsonData
				continue
			case MsgHandMatch:

				pendingNum := b.Queue.GetPendingLen(b.GiantID)
				msg.Content = "0"
				if pendingNum > 0 {
					res = b.Queue.JoinFront(b.GiantID, b.BackUser)
					if res == 1 {
						msg.Content = "1"
					}
				}
				jsonData, _ = json.Marshal(msg)
				Logs.Info("back ->SendFrontPump  MsgHandMatch ip:%s Account:%s  res:%d  msg  :%v", ip, b.UserName, res, msg)
				b.SendBackCMD <- jsonData

				continue
			default:
				Logs.Info("back ->SendFrontPump type:%d", msg.MType)
			}
			//获取前台用户
			tickID := strings.TrimSpace(msg.QueueID)
			if tickID == "" && tickID == "0" {
				closeCode = CloseTickerIDError
				Logs.Error("back-> SendFrontPump GiantID:%d  QueueID:%d   ip:%s   cs:%s  error: Tickerid is null", msg.GiantID, msg.QueueID, ip, b.UserName)
				return
			}
			fqid := ConvTickeridToQueueID(tickID, queueIdKey)
			key := b.Queue.MakeQueueKey(b.GiantID, fqid, b.QueueID)
			u := b.Queue.GetQueueFrontUserByKey(key)
			if u == nil {
				closeCode = CloseTickerIDNotFind
				msg.MType = MsgTypeDown
				msg.Content = fmt.Sprintf("%d", closeCode)
				message, _ = json.Marshal(msg)
				b.SendBackCMD <- message
				Logs.Error("back-> SendFrontPump GiantID:%d  QueueID:%d  ip:%s   cs:%s  error: Tickerid not find", msg.GiantID, msg.QueueID, ip, b.UserName)
				b.SubCurPlayNum()
				continue
				//return
			}
			if msg.MType == MsgServiceEnd {
				u.IsServiceEnd = true
			}

			msg.Timestamp = time.Now().Unix()
			if u.FWS != nil {
				//web socket
				message, _ = json.Marshal(msg)
				u.SendFrontData <- message
			} else {
				//long poll
				r := b.LPmsgPush(u.QueueID, msg)
				Logs.Info("back ->SendFrontPump LPmsgPush  res:%d  GiantID:%d   QueueID:%d  ip:%s   cs:%s  ", r, msg.GiantID, msg.QueueID, ip, b.UserName)
				if r {
					select {
					case u.IsLPNewMsg <- true:
					case <-time.After(1 * time.Second):
					}
				} else {
					msg.MType = MsgTypeDown
					message, _ = json.Marshal(msg)
					b.SendBackCMD <- message
					b.Queue.ExitFront(u)
				}

			}

		case <-time.After(forWaitTime):
			// pendingNumMsg.MType = MsgPendingNum
			// pendingNumMsg.Content = fmt.Sprintf("%d", b.Queue.GetPendingLen(b.GiantID))
			// jsonData, _ = json.Marshal(pendingNumMsg)
			// Logs.Info("back ->SendFrontPump  time.After  MsgPendingNum ip:%s Account:%s msg  :%v", ip, b.UserName, pendingNumMsg)
			// b.SendBackCMD <- jsonData
		}
	}
}
