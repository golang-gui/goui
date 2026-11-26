package main

import (
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/widgets"
)

// 模拟数据需要启动即填满历史，且数值始终落在图表与卡片约定的范围内。
func TestSimulatorKeepsBoundedFullHistory(t *testing.T) {
	s := newSimulator()
	for range 500 {
		s.step()
		for name, history := range map[string][]float32{"cpu": s.cpu, "up": s.up, "down": s.down} {
			if len(history) != historyLength {
				t.Fatalf("%s 历史长度 = %d，期望 %d", name, len(history), historyLength)
			}
		}
		r := s.current
		if r.CPU < 3 || r.CPU > 98 || r.Memory <= 0 || r.Memory >= memoryTotal || r.Up <= 0 || r.Down <= 0 {
			t.Fatalf("读数越界：%+v", r)
		}
		if got := s.cpu[historyLength-1]; got != r.CPU {
			t.Fatalf("最新 CPU 样本 = %v，期望 %v", got, r.CPU)
		}
	}
}

// 排序返回独立副本；未排序或数值相同时按 PID 稳定排列，表格行 ID 不随刷新漂移。
func TestSortedProcessesUseStableOrder(t *testing.T) {
	s := newSimulator()
	items := s.sortedProcesses("cpu", widgets.SortDescending)
	for i := 1; i < len(items); i++ {
		if items[i-1].CPU < items[i].CPU {
			t.Fatalf("CPU 降序在 %d 处失效：%v < %v", i, items[i-1].CPU, items[i].CPU)
		}
	}
	items[0].Name = "changed"
	if s.processes[0].Name == "changed" {
		t.Fatal("排序结果与模拟器共享底层数组")
	}
	none := s.sortedProcesses("", widgets.SortNone)
	for i := 1; i < len(none); i++ {
		if none[i-1].PID >= none[i].PID {
			t.Fatalf("未排序时 PID 应严格递增：%d >= %d", none[i-1].PID, none[i].PID)
		}
	}
	model := processModel{gui.NewSliceListModel(none)}
	if got, want := model.RowID(0), "1200"; got != want {
		t.Fatalf("RowID(0) = %q，期望 %q", got, want)
	}
}

func TestFormatUnits(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{formatMemory(512), "512 MB"},
		{formatMemory(1536), "1.5 GB"},
		{formatRate(.5), "512 KB/s"},
		{formatRate(2.34), "2.3 MB/s"},
	} {
		if tc.got != tc.want {
			t.Errorf("格式化结果 = %q，期望 %q", tc.got, tc.want)
		}
	}
}
