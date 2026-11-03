package diagnostic

import (
	"reflect"
	"testing"
	"time"
)

type countingSource struct{ queries int }

func (s *countingSource) Snapshot(time.Time) GUIData {
	s.queries++
	return GUIData{Roots: []Root{{Ref: 1, ID: "root"}}}
}

func TestSessionSourceQueriedOnlyOnDemand(t *testing.T) {
	s, err := StartCapture(new(int), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	source := &countingSource{}
	s.SetSource(source)
	for range 100 {
		Record(LayoutCreate, 1)
	}
	if source.queries != 0 {
		t.Fatal("记录资源调用了 GUI 数据提供者")
	}
	first := s.Snapshot()
	first.Roots[0].ID = "modified"
	if source.queries != 1 || s.Snapshot().Roots[0].ID != "root" {
		t.Fatal("查询未生成独立结果")
	}
	s.Stop()
	n := source.queries
	frozen := s.Snapshot()
	copy := s.Snapshot()
	copy.Roots[0].ID = "changed"
	if !reflect.DeepEqual(frozen, s.Snapshot()) || source.queries != n || s.Source() != nil || s.Owner() != nil {
		t.Fatal("冻结结果仍调用提供者或借出了数据")
	}
}

func TestFrameHistoryIdleAndRestartSamples(t *testing.T) {
	s, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	r := &RootRecord{Root: Root{Ref: s.NextRef()}}
	for range HistoryLimit + 7 {
		f := s.BeginFrame(r)
		f.EndReturned, f.Completed = true, true
		f.Finish()
	}
	if len(s.Snapshot().Frames) != HistoryLimit || r.Completed != HistoryLimit+7 {
		t.Fatal("历史没有保持上界")
	}
	if got := r.Snapshot(time.Now().Add(2*time.Second), false, false); !got.Idle || got.FPS != 0 {
		t.Fatal("空闲仍报告 FPS", got)
	}
	// 模拟恢复绘制前的一段真实空闲，避免用 sleep 等待。
	r.times[(r.next-1+len(r.times))%len(r.times)] = time.Now().Add(-2 * time.Second)
	f := s.BeginFrame(r)
	f.Completed, f.EndReturned = true, true
	f.Finish()
	if got := r.Snapshot(time.Now(), false, false); got.Idle || got.FPS != 0 || r.count != 1 {
		t.Fatal("恢复后沿用了空闲前的频率", got)
	}
	finished := r.Completed
	f = s.BeginFrame(r)
	s.Stop()
	f.Completed = true
	f.Finish()
	if r.Completed != finished {
		t.Fatal("停止后发布了在途帧")
	}
}
