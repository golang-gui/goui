package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"
)

// 所有原生观察、GUI 输入和日志操作均在 GUI 线程同步执行。
type traceLog struct {
	file                 *os.File
	writer               *bufio.Writer
	start                time.Time
	sequence, lastNative uint64
	scale                float32
	err                  error
}

func newTraceLog(path, note string) (*traceLog, error) {
	var f *os.File
	var err error
	if path == "" {
		f, err = os.CreateTemp("", "goui-input-trace-*.log")
	} else {
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if err != nil {
		return nil, err
	}
	l := &traceLog{file: f, writer: bufio.NewWriterSize(f, 64*1024), start: time.Now()}
	l.record("START", "version=1 os=%s arch=%s go=%s time=%s note=%q", runtime.GOOS, runtime.GOARCH, runtime.Version(), l.start.Format(time.RFC3339Nano), note)
	for _, name := range []string{"GOUI_PLAT_PAINTER", "GOUI_PLAT_SCALE", "DISPLAY", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "SESSIONNAME", "SSH_CONNECTION"} {
		l.record("ENV", "%s=%q", name, os.Getenv(name))
	}
	l.flush()
	return l, nil
}

func (l *traceLog) record(kind, format string, args ...any) {
	if l.err != nil {
		return
	}
	l.sequence++
	if kind == "NATIVE" {
		l.lastNative = l.sequence
	}
	_, l.err = fmt.Fprintf(l.writer, "%06d +%012dus %-6s %s\n", l.sequence, time.Since(l.start).Microseconds(), kind, fmt.Sprintf(format, args...))
}

func (l *traceLog) flush() {
	if l.err == nil {
		l.err = l.writer.Flush()
	}
}

func (l *traceLog) close() error {
	l.flush()
	return errors.Join(l.err, l.file.Close())
}
