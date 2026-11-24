package cocoa

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func TestNativeMenuSnapshotStructure(t *testing.T) {
	a := []common.MenuItem{{ID: 1, Submenu: true, Children: []common.MenuItem{{ID: 2, Title: "Old"}}}}
	b := cloneMenuItems(a)
	b[0].Children[0].Title = "New"
	b[0].Children[0].Enabled = true
	if !sameMenuStructure(a, b) || a[0].Children[0].Title != "Old" {
		t.Fatal("状态更新被当作结构变化或副本共享了可变子切片")
	}
	b[0].Children[0].ID = 3
	if sameMenuStructure(a, b) {
		t.Fatal("项身份替换未判为结构变化")
	}
	b = cloneMenuItems(a)
	b[0].Children[0].Submenu = true
	if sameMenuStructure(a, b) {
		t.Fatal("空子菜单与叶项未区分")
	}
}

func TestNativeMenuShortcutConversion(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key       common.Key
		modifiers common.Modifiers
		text      string
		mask      int
	}{
		{"字母", common.KeyS, common.ModifierCommand, "s", 1 << 20},
		{"组合修饰", common.Key1, common.ModifierCommand | common.ModifierShift | common.ModifierControl | common.ModifierOption, "1", 1<<20 | 1<<17 | 1<<18 | 1<<19},
		{"F1", common.KeyF1, 0, "\uf704", 0},
		{"F24", common.KeyF24, 0, "\uf71b", 0},
		{"左方向", common.KeyArrowLeft, 0, "\uf702", 0},
		{"退格", common.KeyBackspace, 0, "\b", 0},
		{"向前删除", common.KeyDelete, 0, "\x7f", 0},
		{"回车", common.KeyEnter, 0, "\r", 0},
		{"无等价键", common.KeyUnknown, common.ModifierCommand, "", 0},
		{"不合并Alt", common.KeyS, common.ModifierAlt, "", 0},
		{"不合并Super", common.KeyS, common.ModifierSuper, "", 0},
		{"不合并Win", common.KeyS, common.ModifierWin, "", 0},
		{"不合并AltGraph", common.KeyS, common.ModifierAltGraph, "", 0},
		{"不混淆数字键盘", common.KeyNumpad1, 0, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, mask := nativeMenuShortcut(common.MenuShortcut{Key: tc.key, Modifiers: tc.modifiers})
			if text != tc.text || int(mask) != tc.mask {
				t.Fatalf("转换结果 %q/%x，期望 %q/%x", text, mask, tc.text, tc.mask)
			}
		})
	}
}

func TestNativeMenuShortcutUsesCurrentMenuAndAncestors(t *testing.T) {
	leaf := common.MenuItem{ID: 2, Enabled: true, Shortcut: common.MenuShortcut{Key: common.KeyS, Modifiers: common.ModifierCommand}}
	m := nativeMenu{menuSnapshot: menuSnapshot{items: []common.MenuItem{{ID: 1, Enabled: true, Submenu: true, Children: []common.MenuItem{leaf}}}, parents: map[objc.ID]uint64{10: 0, 11: 1}}}
	if !m.shortcutInMenu(Cast[NSMenu](11), common.KeyS, common.ModifierCommand) {
		t.Fatal("已登记等价键未匹配")
	}
	if m.shortcutInMenu(Cast[NSMenu](10), common.KeyS, common.ModifierCommand) || m.shortcutInMenu(Cast[NSMenu](12), common.KeyS, common.ModifierCommand) {
		t.Fatal("等价键抢占了其他菜单或基本应用菜单")
	}
	if m.shortcutInMenu(Cast[NSMenu](11), common.KeyS, common.ModifierControl) {
		t.Fatal("修饰位未精确匹配")
	}
	m.items[0].Enabled = false
	if m.shortcutInMenu(Cast[NSMenu](11), common.KeyS, common.ModifierCommand) {
		t.Fatal("禁用祖先仍匹配")
	}
	m.items[0].Enabled = true
	m.items[0].Children[0].Shortcut.Key = common.KeyF6
	if m.shortcutInMenu(Cast[NSMenu](11), common.KeyS, common.ModifierCommand) || !m.shortcutInMenu(Cast[NSMenu](11), common.KeyF6, common.ModifierCommand) {
		t.Fatal("原位更新仍使用旧等价键")
	}
}
