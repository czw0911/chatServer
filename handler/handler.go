package handler

import (
	"net/http"
	"strconv"
)

func ServerOutputJson(out http.ResponseWriter, data []byte) (int, error) {
	out.Header().Set("Content-Type", "application/json;charset=UTF-8")
	out.Header().Set("Content-Length", strconv.Itoa(len(data)))
	return out.Write(data)
}
