//
// 验证登陆令牌
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-07-25 15:12:45
//

package libs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	//"github.com/astaxie/beego/orm"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	//"time"
)

var AllowOrigin string
var GetKeyServerUrl string

//获取appkey
func GetAppidKey(giantid, appid int64) ([]byte, error) {
	urlVal := &url.Values{}
	urlVal.Set("uid", strconv.FormatInt(giantid, 10))
	urlVal.Set("appid", strconv.FormatInt(appid, 10))
	urlVal.Set("target", "getAppKey")
	url := fmt.Sprintf("%s?%s", GetKeyServerUrl, urlVal.Encode())
	res, err := HttpGet(url)
	return res, err
}

func HttpGet(url string) ([]byte, error) {
	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ztgameChatServer")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if len(body) <= 0 {
		return nil, errors.New("get data is null")
	}
	return body, nil

}

//检查sign
func CheckSign(msg, sign, key string) bool {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	h := mac.Sum(nil)
	code, err := hex.DecodeString(sign)
	if err != nil {
		return false
	}
	return hmac.Equal(code, h)
}

func CheckOrigin(origin string) bool {
	if AllowOrigin == "" || AllowOrigin == "*" {
		return true
	}
	url, err := url.Parse(origin)
	if err != nil {
		return false
	}
	arrAllowOrigin := strings.Split(AllowOrigin, ",")
	for _, v := range arrAllowOrigin {
		if url.Host == v {
			return true
		}
	}
	return false
}
