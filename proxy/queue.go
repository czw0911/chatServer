//
// 用户队列
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-21 16:39:34
//
package proxy

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

func NewQueue() *Queue {
	//处理中队列最大数
	maxNum := MaxDevNum * MaxDevPendingNum
	return &Queue{
		fpq:          make(map[int64][]*FrontUser, MaxDevNum),
		fpqMaxNum:    MaxDevNum,
		fpqDevMaxNum: MaxDevPendingNum,
		fpqLock:      new(sync.RWMutex),
		fq:           make(map[string]*FrontUser, maxNum),
		fLock:        new(sync.RWMutex),
		bq:           make(map[int64][]*BackUser, MaxDevNum),
		bLock:        new(sync.RWMutex),
		fqMaxNum:     maxNum,
		bqMaxNUM:     MaxDevNum,
		bqCSMaxNUM:   MaxDevCSNum,
		fIPcount:     make(map[string]int, 1<<17),
		bIPcount:     make(map[string]int, 1<<17),
		maxIPconn:    MaxIPconn,
		fIPLock:      new(sync.RWMutex),
		bIPLock:      new(sync.RWMutex),
	}
}

type Queue struct {
	//待处理队列
	fpq map[int64][]*FrontUser
	//待处理队列最大数
	fpqMaxNum int
	//待处理队列单个开发者的最大玩家数
	fpqDevMaxNum int
	//待处理队列锁
	fpqLock *sync.RWMutex
	//处理中队列
	fq map[string]*FrontUser
	//处理中队列最大长度
	fqMaxNum int
	//前台处理中队列锁
	fLock *sync.RWMutex
	//前台ip地址计数
	fIPcount map[string]int
	fIPLock  *sync.RWMutex
	//后台队列
	bq map[int64][]*BackUser
	//后台队列最大长度
	bqMaxNUM int
	//后台队列单个开发者允许最大客服数
	bqCSMaxNUM int
	//后台队列锁
	bLock *sync.RWMutex
	//后台ip计数
	bIPcount map[string]int
	bIPLock  *sync.RWMutex
	//单个ip最大连接数
	maxIPconn int
}

//生成队列键值
//GiantID 巨人id FQueueID 前台玩家排队号; BQueueID 后台用户排队号
func (q *Queue) MakeQueueKey(GiantID, FQueueID, BQueueID int64) string {

	return fmt.Sprintf("%d_%d_%d", GiantID, FQueueID, BQueueID)
}

//前台ip计数增加
func (q *Queue) FIPAdd(ip string) bool {
	q.fIPLock.Lock()
	defer q.fIPLock.Unlock()
	//fmt.Printf("fip add:%+v", q.fIPcount)
	if _, ok := q.fIPcount[ip]; ok {
		if q.fIPcount[ip] > q.maxIPconn {
			return false
		}
		q.fIPcount[ip] += 1
	} else {
		q.fIPcount[ip] = 1
	}
	return true
}

//前台ip计数减少
func (q *Queue) FIPSub(ip string) {
	q.fIPLock.Lock()
	defer q.fIPLock.Unlock()
	if _, ok := q.fIPcount[ip]; ok {
		if q.fIPcount[ip] > 0 {
			q.fIPcount[ip] -= 1
		} else {
			delete(q.fIPcount, ip)
		}
	}
	//fmt.Printf("fip sub:%+v", q.fIPcount)
}

//后台ip计数增加
func (q *Queue) BIPAdd(ip string) bool {
	q.bIPLock.Lock()
	defer q.bIPLock.Unlock()
	//fmt.Printf("bip add:%+v", q.bIPcount)
	if _, ok := q.bIPcount[ip]; ok {
		if q.bIPcount[ip] > q.maxIPconn {
			return false
		}
		q.bIPcount[ip] += 1
	} else {
		q.bIPcount[ip] = 1
	}
	return true
}

//后台ip计数减少
func (q *Queue) BIPSub(ip string) {
	q.bIPLock.Lock()
	defer q.bIPLock.Unlock()
	if _, ok := q.bIPcount[ip]; ok {
		if q.bIPcount[ip] > 0 {
			q.bIPcount[ip] -= 1
		} else {
			delete(q.bIPcount, ip)
		}
	}
	//fmt.Printf("bip sub:%+v", q.bIPcount)
}

//加入待处理队列
func (q *Queue) JoinPending(f *FrontUser) int8 {
	q.fpqLock.Lock()
	defer q.fpqLock.Unlock()
	if len(q.fpq) > q.fpqMaxNum {
		return 2 //待处理队列满
	}
	if _, ok := q.fpq[f.GiantID]; ok {
		if len(q.fpq[f.GiantID]) <= q.fpqDevMaxNum {
			q.fpq[f.GiantID] = append(q.fpq[f.GiantID], f)
			return 1
		}
		return 0 //开发者队列满
	} else {

		q.fpq[f.GiantID] = make([]*FrontUser, 0, q.fpqDevMaxNum)
		q.fpq[f.GiantID] = append(q.fpq[f.GiantID], f)
		return 1
	}

}

//删除待处理队列用户
func (q *Queue) DelPendingUser(f *FrontUser) {
	q.fpqLock.Lock()
	defer q.fpqLock.Unlock()
	if _, ok := q.fpq[f.GiantID]; ok {
		qlen := len(q.fpq[f.GiantID])

		if qlen > 0 {
			key := 0
			for k, v := range q.fpq[f.GiantID] {

				if v.Account == f.Account {
					key = k + 1
					qlen = len(q.fpq[f.GiantID])
					if key > qlen {
						key = qlen
					}
					//fmt.Printf("DelPendingUser: %#v  \t len:%d\n", q.fpq[f.GiantID], key)
					q.fpq[f.GiantID] = append(q.fpq[f.GiantID][:k], q.fpq[f.GiantID][key:]...)
				}
			}
		} else {
			delete(q.fpq, f.GiantID)
		}
	}
}

//FIFO退出待处理队列
func (q *Queue) exitPending(GiantID int64) *FrontUser {
	q.fpqLock.Lock()
	defer q.fpqLock.Unlock()
	if _, ok := q.fpq[GiantID]; ok {
		qlen := len(q.fpq[GiantID])
		if qlen > 0 {
			//vip优先
			for k, v := range q.fpq[GiantID] {
				if v.Utype == USER_ACCOUNT_TYPE_VIP {
					q.fpq[GiantID] = append(q.fpq[GiantID][:k], q.fpq[GiantID][k+1:]...)
					if !v.IsOffline {
						return v
					}
				}
			}
			for key, val := range q.fpq[GiantID] {
				if !val.IsOffline {
					out := val
					q.fpq[GiantID] = q.fpq[GiantID][key+1:]
					return out
				}
			}
		}
	}
	return nil
}

//检查队列重复账号,不重复返回false
func (q *Queue) CheckRepeatAccount(giantID int64, account string) (bool, *FrontUser) {
	q.fpqLock.Lock()
	defer q.fpqLock.Unlock()
	if _, ok := q.fpq[giantID]; ok {
		for _, v := range q.fpq[giantID] {

			if v.Account == account {
				return true, v
			}
		}
	}
	for _, v := range q.fq {

		if v.Account == account && v.GiantID == giantID {
			return true, v
		}
	}

	return false, nil
}

//获取待处理队列单个开发者总长度
func (q *Queue) GetPendingLen(giantID int64) int {
	q.fpqLock.Lock()
	defer q.fpqLock.Unlock()
	if _, ok := q.fpq[giantID]; ok {
		return len(q.fpq[giantID])
	}
	return 0
}

//获取待处理队列用户前面人数
func (q *Queue) GetUserPendingNum(giantID int64, account string) int {
	q.fpqLock.Lock()
	defer q.fpqLock.Unlock()
	if _, ok := q.fpq[giantID]; ok {
		for k, v := range q.fpq[giantID] {
			if v.Account == account {
				return k
			}
		}
	}
	return 0
}

//加入处理中队列
func (q *Queue) JoinFront(giantID int64, bUser *BackUser) int8 {
	q.fLock.Lock()
	defer q.fLock.Unlock()
	var u *BackUser
	isHandMatch := false
	if len(q.fq) > q.fqMaxNum {
		return 0 //队列满
	}
	if bUser != nil {
		//手动配对
		u = bUser
		isHandMatch = true
	} else {
		//自动配对
		u = q.matchBackAndFront(giantID)
		if u == nil {
			return 2 //没有后台用户在线
		}
	}
	res := u.AddCurPlayNum(isHandMatch)
	if !res {
		return 3 //后台用户繁忙
	}
	f := q.exitPending(giantID)
	if f == nil {
		return 5 //账号不存在待处理队列中
	}
	// //去重
	// for _, v := range q.fq {
	// 	if v.Account == f.Account {
	// 		return 4 //重复账号
	// 	}
	// }
	f.BUser = u
	//生成处理中队列凭证号
	f.QueueID = time.Now().UnixNano()
	key := q.MakeQueueKey(f.GiantID, f.QueueID, f.BUser.QueueID)
	q.fq[key] = f
	return 1
}

//退出处理中队列
func (q *Queue) ExitFront(f *FrontUser) {
	q.fLock.Lock()
	defer q.fLock.Unlock()
	if f == nil || f.BUser == nil {
		return
	}
	key := q.MakeQueueKey(f.GiantID, f.QueueID, f.BUser.QueueID)
	if _, ok := q.fq[key]; ok {
		delete(q.fq, key)
	}
}

//获取处理中队列里的用户
func (q *Queue) GetQueueFrontUserByKey(key string) *FrontUser {
	q.fLock.RLock()
	defer q.fLock.RUnlock()
	if v, ok := q.fq[key]; ok {
		return v
	}
	return nil
}

// //获取与前台配对的用户。GiantID: 巨人id ; BQueueID :后台用户排队号
// func (q *Queue) GetMatchQueueFrontUser(GiantID, BQueueID int64) []*FrontUser {
// 	q.fLock.RLock()
// 	defer q.fLock.RUnlock()
// 	g := fmt.Sprintf("%d_", GiantID)
// 	bq := fmt.Sprintf("_%d", BQueueID)
// 	arrRes := make([]*FrontUser, 0)
// 	for k, v := range q.fq {
// 		if strings.HasPrefix(k, g) && strings.HasSuffix(k, bq) {
// 			arrRes = append(arrRes, v)
// 		}
// 	}
// 	return arrRes
// }

//获取与后台配对的用户。GiantID: 巨人id ; FQueueID :前台用户排队号
func (q *Queue) GetMatchFQFrontUser(GiantID, FQueueID int64) *FrontUser {
	q.fLock.RLock()
	defer q.fLock.RUnlock()
	g := fmt.Sprintf("%d_%d_", GiantID, FQueueID)
	for k, v := range q.fq {
		if strings.HasPrefix(k, g) {
			return v
		}
	}
	return nil
}

//加入后台队列;每个grant id生成一个队列，队列满返回false
func (q *Queue) JoinBack(b *BackUser) bool {
	q.bLock.Lock()
	defer q.bLock.Unlock()
	if len(q.bq) > q.bqMaxNUM {
		return false
	}
	if _, ok := q.bq[b.GiantID]; ok {
		if len(q.bq[b.GiantID]) <= q.bqCSMaxNUM {
			//删除后台队列重复账号
			for k, v := range q.bq[b.GiantID] {
				if v.UserName == b.UserName {
					q.bq[b.GiantID] = append(q.bq[b.GiantID][:k], q.bq[b.GiantID][k+1:]...)
				}
			}
			//生成队列凭证号
			b.QueueID = time.Now().UnixNano()
			q.bq[b.GiantID] = append(q.bq[b.GiantID], b)
			return true
		}
		return false
	} else {
		//生成排队号
		b.QueueID = time.Now().UnixNano()
		q.bq[b.GiantID] = make([]*BackUser, 0, q.bqCSMaxNUM)
		q.bq[b.GiantID] = append(q.bq[b.GiantID], b)
		return true
	}
}

//退出后台队列
func (q *Queue) ExitBack(b *BackUser) {
	q.bLock.Lock()
	defer q.bLock.Unlock()
	if _, ok := q.bq[b.GiantID]; ok {

		qlen := len(q.bq[b.GiantID])
		if qlen > 0 {
			for k, v := range q.bq[b.GiantID] {
				if v == b {
					q.bq[b.GiantID] = append(q.bq[b.GiantID][:k], q.bq[b.GiantID][k+1:]...)
				}
			}
		} else {
			delete(q.bq, b.GiantID)
		}
	}
}

//前台GiantID与后台配对(FIFO)
func (q *Queue) matchBackAndFront(GiantID int64) *BackUser {
	q.bLock.Lock()
	defer q.bLock.Unlock()
	//fmt.Printf("matchBackAndFront: %#v\n \t giantid:%d", q.bq, GiantID)
	if _, ok := q.bq[GiantID]; ok {
		qlen := len(q.bq[GiantID])
		//println("matchBackAndFront qlen:", qlen)
		if qlen > 1 {
			//先进先出，循环分配
			b := q.bq[GiantID][0]
			q.bq[GiantID] = append(q.bq[GiantID][1:], b)
			return b
		} else if qlen == 1 {
			return q.bq[GiantID][0]
		}
	}
	return nil
}

//检查后台队列重复账号,不重复返回false
func (q *Queue) CheckBackQRepeatAccount(giantID int64, account string) (bool, *BackUser) {
	if _, ok := q.bq[giantID]; ok {
		for _, v := range q.bq[giantID] {
			if v.UserName == account {
				return true, v
			}
		}
	}
	return false, nil
}
