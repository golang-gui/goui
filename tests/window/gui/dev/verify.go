package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/dev"
)

// This verifier only consumes detached protocol data. It never reads GUI
// objects on the HTTP goroutine or invokes editor/model methods.
type nodeInfo struct {
	gui.WidgetInfo
	Ref      string     `json:"ref"`
	Children []nodeInfo `json:"children"`
}

type popupInfo struct {
	Ref    string    `json:"ref"`
	Widget *nodeInfo `json:"widget"`
}

type windowInfo struct {
	Ref      string      `json:"ref"`
	ID       string      `json:"id"`
	Widget   *nodeInfo   `json:"widget"`
	Popovers []popupInfo `json:"popovers"`
}

type protocolClient struct {
	addr string
	http *http.Client
}

func (c protocolClient) request(path string, body any, want int, output any) error {
	method := http.MethodGet
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader, method = bytes.NewReader(data), http.MethodPost
	}
	request, err := http.NewRequest(method, c.addr+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		data, _ := io.ReadAll(response.Body)
		return fmt.Errorf("%s: status=%d want=%d: %s", path, response.StatusCode, want, data)
	}
	if output != nil {
		return json.NewDecoder(response.Body).Decode(output)
	}
	return nil
}

func (c protocolClient) snapshot() (windowInfo, error) {
	var response struct {
		Snapshot struct {
			Windows []windowInfo `json:"windows"`
		} `json:"snapshot"`
	}
	if err := c.request(dev.SnapshotPath, nil, 200, &response); err != nil {
		return windowInfo{}, err
	}
	if len(response.Snapshot.Windows) != 1 {
		return windowInfo{}, fmt.Errorf("expected one window")
	}
	return response.Snapshot.Windows[0], nil
}

func findNode(node *nodeInfo, name string) *nodeInfo {
	if node == nil {
		return nil
	}
	if node.Name == name {
		return node
	}
	for i := range node.Children {
		if found := findNode(&node.Children[i], name); found != nil {
			return found
		}
	}
	return nil
}

func findMenuItem(node *nodeInfo) *nodeInfo {
	if node == nil {
		return nil
	}
	if node.Role == gui.RoleMenuItem && node.Text == "菜单操作" {
		return node
	}
	for i := range node.Children {
		if found := findMenuItem(&node.Children[i]); found != nil {
			return found
		}
	}
	return nil
}

func (c protocolClient) click(target string, node *nodeInfo) error {
	if node == nil || node.Bounds.Width <= 0 || node.Bounds.Height <= 0 {
		return fmt.Errorf("click target has no layout")
	}
	for _, kind := range []string{"pointer_down", "pointer_up"} {
		input := map[string]any{"target": target, "type": kind, "button": "left", "x": node.Bounds.X + node.Bounds.Width/2, "y": node.Bounds.Y + node.Bounds.Height/2}
		if kind == "pointer_down" {
			input["buttons"] = []string{"left"}
		}
		if err := c.request(dev.EventPath, input, 200, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c protocolClient) key(target, key string, primary bool) error {
	for _, kind := range []string{"key_down", "key_up"} {
		input := map[string]any{"target": target, "type": kind, "key": key}
		if primary {
			modifier := "ctrl"
			if runtime.GOOS == "darwin" {
				modifier = "command"
			}
			input["modifiers"] = []string{modifier}
		}
		if err := c.request(dev.EventPath, input, 200, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c protocolClient) text(target, text string, want int) error {
	return c.request(dev.TextPath, map[string]any{"target": target, "text": text}, want, nil)
}

func (c protocolClient) dismiss(popupRef, ownerRef string) error {
	if err := c.request(dev.EventPath, map[string]any{"target": popupRef, "type": "key_down", "key": "escape"}, 200, nil); err != nil {
		return err
	}
	return c.request(dev.EventPath, map[string]any{"target": ownerRef, "type": "key_up", "key": "escape"}, 200, nil)
}

func (c protocolClient) wait(ready func(windowInfo) bool) (windowInfo, error) {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		snapshot, err := c.snapshot()
		if err != nil {
			return windowInfo{}, err
		}
		if ready(snapshot) {
			return snapshot, nil
		}
		select {
		case <-deadline.C:
			return windowInfo{}, fmt.Errorf("window state did not settle")
		case <-tick.C:
		}
	}
}

func verifyProtocol(addr string) error {
	c := protocolClient{addr, &http.Client{Timeout: 3 * time.Second}}
	w, err := c.wait(func(w windowInfo) bool { n := findNode(w.Widget, "main-editor"); return n != nil && n.Bounds.Width > 0 })
	if err != nil {
		return err
	}
	if w.ID != "" || w.Ref == "" {
		return fmt.Errorf("unnamed window has no dev ref")
	}
	if err := c.click(w.Ref, findNode(w.Widget, "main-editor")); err != nil {
		return err
	}
	if err := c.key(w.Ref, "a", true); err != nil {
		return err
	}
	if err := c.text(w.Ref, "你👋e\u0301", 200); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if findNode(w.Widget, "main-editor").Text != "你👋e\u0301" {
		return fmt.Errorf("Unicode selection replacement failed")
	}
	if err := c.key(w.Ref, "z", true); err != nil {
		return err
	}
	if err := c.text(w.Ref, "", 200); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if findNode(w.Widget, "main-editor").Text != "abc" {
		return fmt.Errorf("undo / empty commit changed text")
	}
	if err := c.click(w.Ref, findNode(w.Widget, "readonly-editor")); err != nil {
		return err
	}
	if err := c.text(w.Ref, "blocked", 400); err != nil {
		return err
	}
	if err := c.click(w.Ref, findNode(w.Widget, "multiline-editor")); err != nil {
		return err
	}
	if err := c.key(w.Ref, "a", true); err != nil {
		return err
	}
	if err := c.text(w.Ref, "中\n文👋", 200); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if findNode(w.Widget, "multiline-editor").TextEditing.VisibleText != "中\n文👋" {
		return fmt.Errorf("multiline text commit failed")
	}
	if err := c.click(w.Ref, findNode(w.Widget, "show-popup")); err != nil {
		return err
	}
	w, err = c.wait(func(w windowInfo) bool {
		if len(w.Popovers) != 1 {
			return false
		}
		n := findNode(w.Popovers[0].Widget, "popup-editor")
		return n != nil && n.Bounds.Width > 0
	})
	if err != nil {
		return err
	}
	p := w.Popovers[0]
	if err := c.text(w.Ref, "owner-blocked", 400); err != nil {
		return err
	}
	if err := c.click(p.Ref, findNode(p.Widget, "popup-editor")); err != nil {
		return err
	}
	if err := c.key(p.Ref, "a", true); err != nil {
		return err
	}
	if err := c.text(p.Ref, "面板👋", 200); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if findNode(w.Popovers[0].Widget, "popup-editor").Text != "面板👋" {
		return fmt.Errorf("popover text commit failed")
	}
	if err := c.dismiss(p.Ref, w.Ref); err != nil {
		return err
	}
	if err := c.text(p.Ref, "stale", 400); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if len(w.Popovers) != 0 {
		return fmt.Errorf("closed popup remained in snapshot")
	}
	if err := c.click(w.Ref, findNode(w.Widget, "show-popup")); err != nil {
		return err
	}
	w, err = c.wait(func(w windowInfo) bool { return len(w.Popovers) == 1 })
	if err != nil {
		return err
	}
	if w.Popovers[0].Ref == p.Ref {
		return fmt.Errorf("reopened popover reused a ref")
	}
	if err := c.dismiss(w.Popovers[0].Ref, w.Ref); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if err := c.click(w.Ref, findNode(w.Widget, "show-menu")); err != nil {
		return err
	}
	w, err = c.wait(func(w windowInfo) bool {
		if len(w.Popovers) != 1 {
			return false
		}
		item := findMenuItem(w.Popovers[0].Widget)
		return item != nil && item.Bounds.Width > 0
	})
	if err != nil {
		return err
	}
	menuRef := w.Popovers[0].Ref
	if err := c.click(menuRef, findMenuItem(w.Popovers[0].Widget)); err != nil {
		return err
	}
	w, err = c.snapshot()
	if err != nil {
		return err
	}
	if len(w.Popovers) != 0 || findNode(w.Widget, "menu-count").Text != "menu=1" {
		return fmt.Errorf("internal menu activation failed")
	}
	fmt.Println("PASS: Unicode, undo, readonly, multiline, modal input, stale refs, built-in menu")
	return nil
}
