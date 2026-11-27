package main

import (
	"strings"
	"testing"
)

// 发送消息后会话移到最前，空白内容被拒绝。
func TestStoreSendMovesConversationToTop(t *testing.T) {
	s := newStore()
	if s.send("ming", "   ", today, "10:40") {
		t.Fatal("空白消息不应发送")
	}
	if !s.send("ming", " 出发了 ", today, "10:40") {
		t.Fatal("发送失败")
	}
	if s.contacts[0].ID != "ming" {
		t.Fatalf("最前会话 = %q，应为 ming", s.contacts[0].ID)
	}
	last := s.find("ming").Messages[len(s.find("ming").Messages)-1]
	if !last.mine() || last.Text != "出发了" || last.Time != "10:40" {
		t.Fatalf("最后一条消息 = %+v", last)
	}
}

// 同一会话同时只保留一条待回复；不可见时回复计入未读，可见时不计。
func TestStoreReplyLifecycle(t *testing.T) {
	s := newStore()
	if !s.expectReply("tanaka") {
		t.Fatal("应可预约回复")
	}
	if s.expectReply("tanaka") {
		t.Fatal("同一会话不应重复预约")
	}
	if got := s.nextSender("tanaka"); got != "田中陽" {
		t.Fatalf("回复者 = %q", got)
	}
	unread := s.find("tanaka").Unread
	if !s.deliverReply("tanaka", today, "10:41", false) {
		t.Fatal("投递失败")
	}
	if s.find("tanaka").Unread != unread+1 {
		t.Fatalf("未读 = %d，应为 %d", s.find("tanaka").Unread, unread+1)
	}
	// 预设回复用尽后不再预约。
	if s.expectReply("tanaka") {
		t.Fatal("回复用尽后不应再预约")
	}

	if !s.expectReply("team") || !s.deliverReply("team", today, "10:42", true) {
		t.Fatal("群聊回复失败")
	}
	team := s.find("team")
	if team.Unread != 3 {
		t.Fatalf("可见会话的回复不应增加未读，得到 %d", team.Unread)
	}
	if last := team.Messages[len(team.Messages)-1]; last.Sender != "周宁" {
		t.Fatalf("群聊回复发送者 = %q", last.Sender)
	}
	s.open("team")
	if team.Unread != 0 {
		t.Fatal("打开会话应清除未读")
	}
}

func TestStoreFilterIgnoresCase(t *testing.T) {
	s := newStore()
	if got := s.filter("  "); len(got) != len(s.contacts) {
		t.Fatalf("空查询应返回全部，得到 %d", len(got))
	}
	if got := s.filter("设计"); len(got) != 1 || got[0].ID != "team" {
		t.Fatalf("filter(设计) = %v", got)
	}
	s.find("ming").Name = "Ming"
	if got := s.filter("MING"); len(got) != 1 || got[0].ID != "ming" {
		t.Fatalf("应忽略大小写，得到 %v", got)
	}
}

func TestPreviewAndLabels(t *testing.T) {
	s := newStore()
	if got := preview(s.find("ming")); got != "你：天气好就去" {
		t.Fatalf("preview = %q", got)
	}
	if got := preview(s.find("team")); !strings.HasPrefix(got, "陈一鸣：") || !strings.HasSuffix(got, "…") {
		t.Fatalf("群聊长预览应带发送者并以省略号结尾，得到 %q", got)
	}
	if got := lastActivity(s.find("lin")); got != "10:36" {
		t.Fatalf("今天的消息显示时间，得到 %q", got)
	}
	if got := lastActivity(s.find("ming")); got != yesterday {
		t.Fatalf("更早的消息显示日期，得到 %q", got)
	}
	if got := initial("alex"); got != "A" {
		t.Fatalf("initial = %q", got)
	}
	c := clock{minutes: 23*60 + 59}
	if got := c.next(); got != "00:00" {
		t.Fatalf("时钟应跨日回绕，得到 %q", got)
	}
}
