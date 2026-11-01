package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceLogOrderAndNativeReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.log")
	l, err := newTraceLog(path, "触摸板\n设备")
	if err != nil {
		t.Fatal(err)
	}
	l.record("NATIVE", "wheel.raw=%d", -30)
	ref := l.lastNative
	l.record("GOUI", "lastNative=%d wheel delta=-0.25,0", ref)
	l.record("STATE", "table.scroll offset=4,0")
	if l.lastNative != ref {
		t.Fatalf("普通记录改变了原生引用：%d != %d", l.lastNative, ref)
	}
	count := l.sequence
	if err := l.close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if uint64(len(lines)) != count {
		t.Fatalf("记录未逐行保存：行数=%d 序号=%d", len(lines), count)
	}
	if !strings.Contains(lines[0], `note="触摸板\n设备"`) {
		t.Fatalf("设备注释未转义：%s", lines[0])
	}
	if !strings.Contains(lines[len(lines)-3], "NATIVE wheel.raw=-30") || !strings.Contains(lines[len(lines)-2], "GOUI   lastNative=") || !strings.Contains(lines[len(lines)-1], "STATE  table.scroll") {
		t.Fatalf("原生、转换、状态顺序错误：%s", data)
	}
}

func TestTraceLogDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.log")
	if err := os.WriteFile(path, []byte("已有硬件记录"), 0600); err != nil {
		t.Fatal(err)
	}
	if l, err := newTraceLog(path, ""); err == nil {
		l.close()
		t.Fatal("错误地覆盖了已有文件")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "已有硬件记录" {
		t.Fatalf("旧日志损坏：%q %v", data, err)
	}
}

func TestTraceLogReportsWriteFailure(t *testing.T) {
	l, err := newTraceLog(filepath.Join(t.TempDir(), "closed.log"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.file.Close(); err != nil {
		t.Fatal(err)
	}
	l.record("NATIVE", "test")
	l.flush()
	if l.err == nil {
		t.Fatal("日志写入失败被忽略")
	}
	first := l.err
	l.record("GOUI", "discarded after failure")
	if l.err != first {
		t.Fatal("原始写入错误被替换")
	}
	if l.close() == nil {
		t.Fatal("关闭时没有报告失败")
	}
}
