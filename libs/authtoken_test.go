//test authtoken
//
//@Author  : chenzhangwei
//@Email   : czw@outlook.com
//@Date    : 2014-09-10 16:21:37
//

package libs

import (
	"fmt"
	"testing"
)

func TestGetAppidKey(t *testing.T) {
	GetKeyServerUrl = "http://wangchao1.dev.ztgame.com/win/userinfo"
	res, err := GetAppidKey(22, 34)
	if err != nil {
		t.Errorf("%v", err)
	}
	fmt.Printf("key: %s\n", res)
}
