package diagnostics

import (
	"reflect"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/diagnostic"
)

// 只作会话归属标识；所有方法都不应被诊断控制路径调用。
type applicationToken struct{ gui.Application }

func TestCaptureControlDoesNotInvokeApplication(t *testing.T) {
	previous := gui.App
	app := &applicationToken{}
	gui.App = app
	t.Cleanup(func() { gui.App = previous })
	capture, err := Start(app, Options{Widgets: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(capture.Stop)
	if diagnostic.Current().Source() != nil {
		t.Fatal("开启时提前创建了 GUI 采集器")
	}
	if _, err := Start(app, Options{}); err == nil {
		t.Fatal("重复开启未报告错误")
	}
	diagnostic.RecordImage(diagnostic.ImageCreate, 3, 2)
	s := capture.Snapshot()
	if !s.Active || !s.Options.Widgets || s.Resources.ImagesCreated != 1 || s.Resources.PixelBytes != 24 || len(s.Frames) != 0 || diagnostic.Current().Source() != nil {
		t.Fatal("查询创建了 GUI 活动或资源统计错误", s)
	}
	diagnostic.StopOwner(&applicationToken{})
	if !capture.Snapshot().Active {
		t.Fatal("其它归属停止了采集")
	}
	diagnostic.StopOwner(app)
	frozen := capture.Snapshot()
	diagnostic.Record(diagnostic.ImageDestroy, 1)
	if frozen.Active || !reflect.DeepEqual(frozen, capture.Snapshot()) || diagnostic.Current() != nil || capture.session.Source() != nil || capture.session.Owner() != nil {
		t.Fatal("退出停止未冻结或未清理归属")
	}
	next, err := Start(app, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Stop()
	capture.Stop()
	if !next.Snapshot().Active || next.Snapshot().Resources.ImagesCreated != 0 {
		t.Fatal("旧句柄停止了新采集或新采集混入旧计数")
	}
}

func TestCaptureRejectsUnregisteredApplication(t *testing.T) {
	previous := gui.App
	gui.App = nil
	t.Cleanup(func() { gui.App = previous })
	for _, app := range []gui.Application{nil, &applicationToken{}} {
		if _, err := Start(app, Options{}); err == nil {
			t.Fatal("未就绪的应用开启了采集")
		}
	}
	gui.App = &applicationToken{}
	if _, err := Start(&applicationToken{}, Options{}); err == nil {
		t.Fatal("接受了非当前应用")
	}
	var empty *Capture
	empty.Stop()
	if empty.Snapshot().Active {
		t.Fatal("空句柄启用采集")
	}
}
