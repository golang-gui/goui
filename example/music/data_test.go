package main

import (
	"testing"
	"time"
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// 播放时位置随时间推进，暂停后保持不变。
func TestPlayerPositionFollowsPlayState(t *testing.T) {
	p := newPlayer()
	if got := p.position(epoch.Add(time.Minute)); got != 0 {
		t.Fatalf("暂停时位置 = %v，应为 0", got)
	}
	p.toggle(epoch)
	if got := p.position(epoch.Add(10 * time.Second)); got != 10*time.Second {
		t.Fatalf("播放 10 秒后位置 = %v", got)
	}
	p.toggle(epoch.Add(10 * time.Second))
	if got := p.position(epoch.Add(time.Minute)); got != 10*time.Second {
		t.Fatalf("暂停后位置 = %v，应保持 10s", got)
	}
}

// 播完自动切到下一首并带上超出的时间；最后一首之后回到第一首。
func TestPlayerAdvanceCarriesOverrunAndWraps(t *testing.T) {
	p := newPlayer()
	p.jump(len(tracks)-1, epoch)
	p.toggle(epoch)
	end := epoch.Add(tracks[len(tracks)-1].Length + 2*time.Second)
	if !p.advance(end) {
		t.Fatal("播完后应切歌")
	}
	if p.index != 0 {
		t.Fatalf("最后一首之后 index = %d，应为 0", p.index)
	}
	if got := p.position(end); got != 2*time.Second {
		t.Fatalf("超出时间未带入下一首：%v", got)
	}
	if p.advance(end) {
		t.Fatal("同一时刻不应再次切歌")
	}
}

// 跳转比例限制在 [0, 1]；序号越界时环绕。
func TestPlayerSeekAndJumpClamp(t *testing.T) {
	p := newPlayer()
	p.seek(1.5, epoch)
	if got := p.position(epoch); got != p.current().Length {
		t.Fatalf("seek(1.5) = %v，应为全长", got)
	}
	p.seek(-1, epoch)
	if got := p.position(epoch); got != 0 {
		t.Fatalf("seek(-1) = %v，应为 0", got)
	}
	p.jump(-1, epoch)
	if p.index != len(tracks)-1 {
		t.Fatalf("jump(-1) index = %d", p.index)
	}
}

// 上一首：已播放超过 3 秒时回到本曲开头，否则切到上一首。
func TestPlayerPreviousRestartsOrGoesBack(t *testing.T) {
	p := newPlayer()
	p.jump(2, epoch)
	p.seek(.5, epoch)
	p.previous(epoch)
	if p.index != 2 || p.position(epoch) != 0 {
		t.Fatalf("应重播本曲，得到 index=%d pos=%v", p.index, p.position(epoch))
	}
	p.previous(epoch)
	if p.index != 1 {
		t.Fatalf("应切到上一首，得到 index=%d", p.index)
	}
}

func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		{0, "0:00"}, {59 * time.Second, "0:59"}, {length(3, 42), "3:42"}, {-time.Second, "0:00"},
	} {
		if got := formatDuration(tc.in); got != tc.want {
			t.Errorf("formatDuration(%v) = %q，应为 %q", tc.in, got, tc.want)
		}
	}
}
