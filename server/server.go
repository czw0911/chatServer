//
// 服务器
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-21 16:39:34
//
package server

import (
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	//监听端口号
	Port int
	//处理逻辑接口
	Handler http.Handler
}

func (s *Server) Run() error {

	addr := fmt.Sprintf(":%d", s.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        s.Handler,
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return srv.ListenAndServe()
}
