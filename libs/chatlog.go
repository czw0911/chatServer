//聊天日志
//
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-08-27 15:09:03
//

package libs

import (
	"fmt"
	"github.com/astaxie/beego/orm"
	"sync"
	"time"
)

const (
	// //聊天信息关闭状态码
	// CLOSE_STATE_INIT     = 0
	// CLOSE_STATE_PLAYER   = 1
	// CLOSE_STATE_CS       = 2
	// CLOSE_STATE_NORMAL   = 3
	// CLOSE_STATE_CS_BUSY  = 4
	// CLOSE_STATE_SYS_BUSY = 5

	//聊天信息发送类型
	SEND_TYPE_PLAYER = 1
	SEND_TYPE_CS     = 2
	SEND_TYPE_SYS    = 3
)

//聊天信息
type ChatInfo struct {
	ChatID         int64
	GiantID        int64
	AppID          int64
	PlayerAccount  string
	PlayerNickName string
	SRVname        string
	RoleName       string
	CSaccount      string
	CSnickName     string
	CloseState     int8
	InTime         int64
	StartTime      int64
	EndTime        int64
	PlayerRating   int8
	RatingTime     int64
	FeedBack       string
}

//聊天信息明细
type ChatInfoDetail struct {
	ChatID   int64
	Player   string
	CsName   string
	SendType int8
	ChatTime int64
	MsgType  int8
	ChatMsg  string
}

//聊天日志
type ChatLog struct {
	CtLock *sync.RWMutex
}

func NewChatLog() *ChatLog {
	return &ChatLog{new(sync.RWMutex)}
}

//创建聊天日志表结构
func (c *ChatLog) CreateChatLogTable() (bool, error) {
	c.CtLock.Lock()
	defer c.CtLock.Unlock()

	tableName := time.Now().Format("20060102")
	sqlStr := `
	CREATE  TABLE  IF NOT EXISTS  chat_log_` + tableName + `   (
	    id   bigint(20) unsigned NOT NULL AUTO_INCREMENT,
	    chat_id   bigint(20) unsigned NOT NULL COMMENT '聊天id',
	    giant_id   bigint(20) unsigned NOT NULL COMMENT '巨人id(开发者uid)',
	    app_id   bigint(20) unsigned NOT NULL COMMENT 'app id',
	    player_account   varchar(50) NOT NULL COMMENT '玩家名',
	    player_nickname   varchar(45) DEFAULT '' COMMENT '玩家昵称',
	    srv_name   varchar(50) DEFAULT '' COMMENT 'app服务器名',
	    role_name   varchar(50) DEFAULT '' COMMENT '玩家角色名',
	    cs_account   varchar(50) NOT NULL COMMENT '客服账号',
	    cs_nickname   varchar(45) DEFAULT '' COMMENT '客服昵称',
	    close_state   int DEFAULT 0 COMMENT '关闭聊天状态码（同web socket close code）',
	    in_time   bigint(20) unsigned DEFAULT 0 COMMENT '接入时间(待处理）',
	    start_time   bigint(20) unsigned DEFAULT 0 COMMENT '开始聊天时间（处理中）',
	    end_time   bigint(20) unsigned DEFAULT 0 COMMENT '结束聊天时间',
	    player_rating   tinyint(4) DEFAULT 0 COMMENT '玩家满意度评分',
	    rating_time   bigint(20) unsigned DEFAULT 0 COMMENT '评分时间',
	    feedback   varchar(5000) DEFAULT '' COMMENT '玩家意见反馈',
	  PRIMARY KEY (  id  ),
	  KEY   chat_log_index   (  chat_id  ,  app_id  ,  giant_id  ,  in_time  ,  start_time  ,  end_time  ,  player_rating  ,  cs_account  ,  player_account  ,  rating_time  )
	) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='聊天日志(日表)'
`
	db := orm.NewOrm()
	_, err := db.Raw(sqlStr).Exec()
	if err != nil {
		return false, err
	}

	sqlStr = `
		CREATE  TABLE   IF NOT EXISTS   chat_log_detail_` + tableName + `   (
		    id   bigint(20) unsigned NOT NULL AUTO_INCREMENT,
		    chat_id   bigint(20) unsigned NOT NULL COMMENT '聊天id',
		    player_nickname   varchar(45) DEFAULT NULL COMMENT '玩家昵称',
		    cs_nickname   varchar(45) DEFAULT NULL COMMENT '客服昵称',
		    send_type   tinyint(4) DEFAULT NULL COMMENT '聊天类型（1,玩家发送；2，客服发送；3，系统发送）',
		    chat_time   bigint(20) unsigned DEFAULT NULL COMMENT '聊天消息发送时间',
		    msg_type   tinyint(4) DEFAULT NULL COMMENT '消息类型',
		    chat_msg   varchar(1024) DEFAULT NULL COMMENT '聊天消息',
		  PRIMARY KEY (  id  ),
		  KEY   chat_detail_index   (  chat_id  )
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='聊天日志明细(日表）'
	`
	_, err = db.Raw(sqlStr).Exec()
	if err != nil {
		return false, err
	}
	return true, nil
}

//根据chatid获取表名后缀
func (c *ChatLog) GetTableSuffix(chatID int64) string {
	t := time.Unix(0, chatID)
	return t.Format("20060102")
}

//添加聊天日志
func (c *ChatLog) AddChatLog(chatID int64, v *ChatInfo) error {
	db := orm.NewOrm()
	tabName := fmt.Sprintf("chat_log_%s", c.GetTableSuffix(chatID))
	sqlstr := `INSERT INTO ` + tabName + ` (chat_id,giant_id,app_id,
		player_account,player_nickname,srv_name,role_name,
		cs_account,cs_nickname,close_state,in_time,start_time,
		end_time,player_rating,rating_time,feedback)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`
	p, err := db.Raw(sqlstr).Prepare()
	defer func() {
		if p != nil {
			p.Close()
		}

	}()
	if err != nil {
		return err
	}

	_, err = p.Exec(v.ChatID, v.GiantID, v.AppID, v.PlayerAccount, v.PlayerNickName, v.SRVname, v.RoleName, v.CSaccount, v.CSnickName, v.CloseState, v.InTime, v.StartTime, v.EndTime, v.PlayerRating, v.RatingTime, v.FeedBack)
	if err != nil {
		return err
	}

	return nil
}

//更新对话开始时间
func (c *ChatLog) UpdateChatStartTime(chatID, startTime int64) error {
	db := orm.NewOrm()
	tabName := fmt.Sprintf("chat_log_%s", c.GetTableSuffix(chatID))
	_, err := db.Raw("UPDATE "+tabName+"  SET start_time=? WHERE chat_id=?", startTime, chatID).Exec()
	if err != nil {
		return err
	}
	return nil
}

//更新消息状态
func (c *ChatLog) UpdateChatState(chatID, endTime int64, closeState int) error {
	db := orm.NewOrm()
	tabName := fmt.Sprintf("chat_log_%s", c.GetTableSuffix(chatID))
	_, err := db.Raw("UPDATE "+tabName+"  SET close_state=? , end_time=? WHERE chat_id=?", closeState, endTime, chatID).Exec()
	if err != nil {
		return err
	}
	return nil
}

//添加聊天日志明细
func (c *ChatLog) AddChatLogDetail(chatID int64, arrParam []*ChatInfoDetail) error {
	db := orm.NewOrm()
	tabName := fmt.Sprintf("chat_log_detail_%s", c.GetTableSuffix(chatID))
	p, err := db.Raw("INSERT INTO " + tabName + " (chat_id,player_nickname,cs_nickname,send_type,chat_time,msg_type,chat_msg)VALUES(?,?,?,?,?,?,?)").Prepare()
	defer func() {
		if p != nil {
			p.Close()
		}

	}()
	if err != nil {
		return err
	}
	for _, v := range arrParam {
		if v == nil || v.ChatMsg == "" {
			continue
		}
		_, err = p.Exec(v.ChatID, v.Player, v.CsName, v.SendType, v.ChatTime, v.MsgType, v.ChatMsg)
		if err != nil {
			return err
		}
	}
	return nil

}
