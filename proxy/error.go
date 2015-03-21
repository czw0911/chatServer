//
// 错误定义
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-23 18:04:45
//

package proxy

//close code扩展
const (
	//
	//通用错误码
	//
	//没有回应心跳包
	CloseRespondCheck = 8000
	//队列已满
	CloseQueueIsFull = 8001
	//web socket访问认证错误
	CloseWSTokenError = 8002
	//消息格式错误
	CloseMsgFormatError = 8003
	//空闲太久
	CloseLinkIdleError = 8004
	//ip格式错误
	CloseIPFormatError = 8005
	//单个ip连接数过多
	CloseIpConnMaxError = 8006
	//账号重复登录
	CloseRepeatLoginError = 8007

	//
	//前台错误码
	//
	//后台没有坐席
	CloseBackNotSeats = 8100
	//后台坐席掉线
	CloseBackSeatsDown = 8101
	//接收后台数据错误
	CloseReceiveBackError = 8102
	//发送后台数据到前台错误
	CloseSendToFrontErr = 8103
	//前台掉线
	CloseFrontDown = 8104
	//接收前台消息错误
	CloseReceiveFrontError = 8105
	//后台坐席繁忙
	CloseSeatsProcFull = 8106
	//服务结束
	CloseServiceEnd = 8107
	//接收控制命令错误
	CloseReveicFrontCMD = 8108
	//账号不存在待处理队列中
	CloseAccountNotFind = 8109
	//tickerID不存在
	CloseTickerIDNotMatch = 8110

	//
	//后台错误码
	//
	//接收后台CHAN数据错误
	CloseReveiceBackChan = 8200
	//后台掉线
	CloseBackDown = 8201
	//tickerID错误
	CloseTickerIDError = 8202
	//tickerID不存在
	CloseTickerIDNotFind = 8203
	//接收前台消息错误
	CloseFrontChanError = 8204
	//消息转发到后台错误
	CloseFrontToBackError = 8205
	//接收后台WS数据错误
	CloseReveiceBackWS = 8206
	//接收控制命令错误
	CloseReveiceBackCMD = 8207
)
