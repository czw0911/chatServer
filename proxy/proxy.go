//
// 消息转发网关
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-21 16:58:10
//

package proxy

import (
	"ChatServer/libs"
	"container/list"
	"encoding/base64"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/gorilla/websocket"
	"strconv"
	"sync"
	"time"
)

const (
	writeWait = 10 * time.Second
	//前台心跳检查间隔
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	//后台心跳检查间隔
	pongWaitBack = 300 * time.Second

	pingPeriodBack = (pongWaitBack * 9) / 10

	maxMessageSize = 1024

	//queueid转换key
	queueIdKey int64 = 0x19C6F3D4DF

	//下一次循环等待时长
	forWaitTime = 100000 * time.Nanosecond
)

//消息类型
const (
	//掉线消息
	MsgTypeDown int8 = 1
	//文字消息
	MsgTypeTxt int8 = 2
	//图片地址消息
	MsgTypeImg int8 = 3
	//服务结束
	MsgServiceEnd int8 = 4
	//查询,返回待处理队列数
	MsgPendingNum int8 = 5
	//手动配对拉待处理队列用户
	MsgHandMatch int8 = 6
)

//账号类型
const (
	//普通用户
	USER_ACCOUNT_TYPE_GENERAL int8 = 1
	//vip账号
	USER_ACCOUNT_TYPE_VIP int8 = 2
)

var (
	//容许最大开发者数
	MaxDevNum = 1000
	//待处理队列的单个开发者最大玩家数
	MaxDevPendingNum = 1000
	//单个客服同时处理最大玩家数（自动配对）
	MaxCSProcNum = 10
	//后台单个开发者允许最大客服数
	MaxDevCSNum = 20
	//单个ip最大连接数
	MaxIPconn = 20
	//loog poll msg max size
	MaxMsgSize = 30
	//系统日志
	Logs *logs.BeeLogger
	//聊天日志
	ChatLogs *libs.ChatLog
)

//通用消息结构
type MsgCommon struct {
	//消息类型
	MType int8 `json:"mtype"`
	//排队号(入场劵)
	QueueID string `json:"ticketid"`
	//巨人id
	GiantID int64 `json:"giantid"`
	//应用id
	AppID int64 `json:"appid"`
	//玩家账号
	Account string `json:"account"`
	//玩家账号类型
	Utype int8 `json:"utype"`
	//玩家昵称
	PlayerName string `json:"player"`
	//客服昵称
	CsName string `json:"cs"`
	//时间戳
	Timestamp int64 `json:"time"`
	//消息内容
	Content string `json:"content"`
}

//前台用户
type FrontUser struct {
	//前台ws连接
	FWS *websocket.Conn
	//配对的后台用户
	BUser *BackUser
	//队列
	Queue *Queue
	//处理中队列凭证号
	QueueID int64
	//巨人id
	GiantID int64
	//应用id
	AppID int64
	//玩家账号
	Account string
	//玩家账号类型 2=vip
	Utype int8
	//玩家昵称
	NickName string
	//游戏服务器名
	SrvName string
	//游戏角色名
	RoleName string
	//需要发送到前台的消息
	SendFrontData chan []byte
	//需要发送到前台的控制命令
	SendFrontCMD chan []byte
	//是否掉线
	IsOffline bool
	//服务是否结束
	IsServiceEnd bool
	//lp msg
	IsLPNewMsg chan bool
	//最后活跃时间期限
	LastActiveTime int64
	Lock           *sync.RWMutex
}

//后台用户
type BackUser struct {
	//后台ws连接
	BWS *websocket.Conn
	//队列凭证号
	QueueID int64
	//巨人id
	GiantID int64
	//应用id
	AppID int64
	//用户名（账号）
	UserName string
	//用户昵称
	NickName string
	//队列
	Queue *Queue
	//需要发送到前台的消息
	SendFrontData chan []byte
	//需要发送到后台的消息
	SendBackData chan []byte
	//需要发送到后台的控制命令
	SendBackCMD chan []byte
	//是否掉线
	IsOffline bool
	//当前服务玩家人数
	CurPlayerNum int
	//lp msg
	LPmsg map[int64]*list.List
	//lp lock
	LPLock    *sync.RWMutex
	Lock      *sync.RWMutex
	LockNum   *sync.RWMutex
	LockMatch *sync.RWMutex
}

//增加一个当前处理玩家数
func (b *BackUser) AddCurPlayNum(isHandMatch bool) bool {
	b.LockNum.Lock()
	defer b.LockNum.Unlock()
	num := b.CurPlayerNum + 1
	if num <= MaxCSProcNum || isHandMatch {
		b.CurPlayerNum = num
		return true
	}
	return false
}

//减少一个当前处理玩家数
func (b *BackUser) SubCurPlayNum() {
	b.LockNum.Lock()
	defer b.LockNum.Unlock()
	b.CurPlayerNum -= 1
	if b.CurPlayerNum < 0 {
		b.CurPlayerNum = 0
	}
}

// push lp msg
func (b *BackUser) LPmsgPush(queueID int64, msg MsgCommon) bool {
	b.LPLock.Lock()
	defer b.LPLock.Unlock()
	t := time.Now().Unix()
	if v, ok := b.LPmsg[queueID]; ok {
		if b.LPmsg[queueID].Len() > 0 {
			m := b.LPmsg[queueID].Front().Value.(MsgCommon)
			subT := t - m.Timestamp
			if subT > 30 {
				//player down
				//delete(b.LPmsg, queueID)
				return false
			}
			if b.LPmsg[queueID].Len() >= MaxMsgSize {
				b.LPmsg[queueID].Remove(b.LPmsg[queueID].Front())
			}
		}
		v.PushBack(msg)
	} else {
		b.LPmsg[queueID] = list.New()
		b.LPmsg[queueID].PushBack(msg)
	}
	return true
}

// pop lp msg
func (b *BackUser) LPmsgPop(queueID int64) []MsgCommon {
	b.LPLock.Lock()
	defer b.LPLock.Unlock()
	arrMsg := make([]MsgCommon, 0, 10)
	if _, ok := b.LPmsg[queueID]; !ok {
		return arrMsg
	}
	if b.LPmsg[queueID].Len() == 0 {
		return arrMsg
	}

	for msg := b.LPmsg[queueID].Front(); msg != nil; msg = msg.Next() {
		arrMsg = append(arrMsg, msg.Value.(MsgCommon))
	}
	b.LPmsg[queueID].Init()
	return arrMsg
}

//获取客服当前处理玩家数
func (b *BackUser) GetCurPlayNum() int {
	// b.LockNum.RLock()
	// defer b.LockNum.RUnlock()
	return b.CurPlayerNum
}

//queueID转换成tickerID
func ConvQueueIDToTickerID(qid, key int64) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%09x%x", time.Now().Nanosecond(), qid^key)))
}

//tickerID转换成queueID
func ConvTickeridToQueueID(tid string, key int64) int64 {
	d, _ := base64.StdEncoding.DecodeString(tid)
	if len(d) > 10 {
		q, _ := strconv.ParseInt(fmt.Sprintf("%s", d[9:]), 16, 64)
		return q ^ key
	}
	return 0
}

func GetConvQueueIdKey() int64 {
	return queueIdKey
}
