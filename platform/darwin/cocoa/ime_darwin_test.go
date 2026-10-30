package cocoa

import (
	"testing"
)

// 无效输入在调用原生接口前拒绝，无需桌面。
func TestInputMethodFactoryErrorHasNoInstance(t *testing.T) {
	p := &Platform{}
	im, err := p.NewInputMethod(nil, nil)
	if err == nil {
		t.Fatal("unavailable input method succeeded")
	}
	if im != nil {
		t.Fatalf("failure returned a non-nil interface holding %T", im)
	}
}
