package main

import (
	"fmt"
	"slices"
	"strings"
)

// message is one chat entry. An empty Sender means the local user.
type message struct {
	Sender string
	Text   string
	Day    string // date group shown as a separator, e.g. "今天"
	Time   string // HH:MM
}

func (m message) mine() bool { return m.Sender == "" }

// reply is a canned answer delivered after the user writes.
type reply struct{ sender, text string }

// contact is a one-to-one or group conversation.
type contact struct {
	ID, Name string
	Color    int            // avatar palette index
	Group    bool           // groups show sender names and avatars
	Members  map[string]int // group member name -> avatar palette index
	Online   bool
	Seen     string // presence shown while offline
	Messages []message
	Unread   int
	replies  []reply
	next     int
	awaiting bool
}

// memberColor returns the avatar color of a message sender.
func (c *contact) memberColor(sender string) int {
	if color, ok := c.Members[sender]; ok {
		return color
	}
	return c.Color
}

// store holds every conversation, most recent first. It is plain data with
// no GUI dependency.
type store struct{ contacts []*contact }

func (s *store) find(id string) *contact {
	for _, c := range s.contacts {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// filter returns conversations whose name contains query, ignoring case.
func (s *store) filter(query string) []*contact {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return s.contacts
	}
	var out []*contact
	for _, c := range s.contacts {
		if strings.Contains(strings.ToLower(c.Name), query) {
			out = append(out, c)
		}
	}
	return out
}

// open marks a conversation as read.
func (s *store) open(id string) {
	if c := s.find(id); c != nil {
		c.Unread = 0
	}
}

// send appends the user's text and moves the conversation to the top.
// Blank text is rejected.
func (s *store) send(id, text, day, at string) bool {
	c := s.find(id)
	text = strings.TrimSpace(text)
	if c == nil || text == "" {
		return false
	}
	c.Messages = append(c.Messages, message{Text: text, Day: day, Time: at})
	s.touch(c)
	return true
}

// expectReply reserves the next canned reply, if any. Only one reply per
// conversation is outstanding at a time.
func (s *store) expectReply(id string) bool {
	c := s.find(id)
	if c == nil || c.awaiting || c.next >= len(c.replies) {
		return false
	}
	c.awaiting = true
	return true
}

// nextSender names who is about to reply.
func (s *store) nextSender(id string) string {
	c := s.find(id)
	if c == nil || c.next >= len(c.replies) {
		return ""
	}
	if sender := c.replies[c.next].sender; sender != "" {
		return sender
	}
	return c.Name
}

// deliverReply appends the reserved reply. It counts as unread unless the
// conversation is currently visible.
func (s *store) deliverReply(id, day, at string, visible bool) bool {
	c := s.find(id)
	if c == nil || !c.awaiting {
		return false
	}
	c.awaiting = false
	sender := s.nextSender(id)
	c.Messages = append(c.Messages, message{Sender: sender, Text: c.replies[c.next].text, Day: day, Time: at})
	c.next++
	if !visible {
		c.Unread++
	}
	s.touch(c)
	return true
}

func (s *store) touch(c *contact) {
	if i := slices.Index(s.contacts, c); i > 0 {
		s.contacts = slices.Insert(slices.Delete(s.contacts, i, i+1), 0, c)
	}
}

// preview is the single-line summary shown in the conversation list.
func preview(c *contact) string {
	if len(c.Messages) == 0 {
		return ""
	}
	last := c.Messages[len(c.Messages)-1]
	text := strings.Join(strings.Fields(last.Text), " ")
	switch {
	case last.mine():
		text = "你：" + text
	case c.Group:
		text = last.Sender + "：" + text
	}
	// ListRow subtitles wrap; keep the preview within one line of the
	// default list width (about nine CJK characters including the ellipsis).
	const limit = 8
	if runes := []rune(text); len(runes) > limit {
		text = string(runes[:limit]) + "…"
	}
	return text
}

// lastMine is the index of the user's newest message, or -1. Only that
// message carries a read receipt.
func lastMine(c *contact) int {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].mine() {
			return i
		}
	}
	return -1
}

// lastActivity is the time label of the newest message.
func lastActivity(c *contact) string {
	if len(c.Messages) == 0 {
		return ""
	}
	last := c.Messages[len(c.Messages)-1]
	if last.Day == today {
		return last.Time
	}
	return last.Day
}

// initial is the avatar letter: the first character, upper-cased.
func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return ""
}

// clock is the simulated wall clock for new messages. It starts just after the
// newest sample message and advances one minute per message, so times stay
// in order whatever the real time is.
type clock struct{ minutes int }

func (c *clock) next() string {
	c.minutes++
	return fmt.Sprintf("%02d:%02d", c.minutes/60%24, c.minutes%60)
}
