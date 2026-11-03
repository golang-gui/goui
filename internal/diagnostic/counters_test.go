package diagnostic

import (
	"sync"
	"testing"
)

func TestCaptureCountersAndRestart(t *testing.T) {
	Record(LayoutCreate, 10)
	s, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	if _, err := Start(); err == nil {
		t.Fatal("重复开启未失败")
	}
	RecordImage(ImageCreate, 3, 2)
	RecordImage(ImageUpdate, 3, 2)
	var jobs sync.WaitGroup
	for range 8 {
		jobs.Add(1)
		go func() {
			defer jobs.Done()
			for range 1000 {
				Record(LayoutCreate, 1)
			}
		}()
	}
	jobs.Wait()
	v := s.Values()
	if v[LayoutCreate] != 8000 || v[ImageCreate] != 1 || v[ImageUpdate] != 1 || v[ImageBytes] != 48 {
		t.Fatal("资源计数或像素载荷错误", v)
	}
	s.Stop()
	Record(LayoutCreate, 100)
	if s.Values() != v {
		t.Fatal("停用后继续采集")
	}
	next, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	defer next.Stop()
	s.Stop() // 旧句柄不能停止新采集。
	Record(ImageDestroy, 1)
	if got := next.Values(); got[ImageDestroy] != 1 || got[LayoutCreate] != 0 {
		t.Fatal("重启混入旧计数", got)
	}
}

func TestDisabledRecordDoesNotAllocate(t *testing.T) {
	if n := testing.AllocsPerRun(1000, func() { Record(LayoutCreate, 1); RecordImage(ImageCreate, 2, 2) }); n != 0 {
		t.Fatal("关闭状态存在分配", n)
	}
}

func BenchmarkRecordDisabled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Record(LayoutCreate, 1)
	}
}
