package x11

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// 固定资源按XSETTINGS v0.5逐字段编码，预期独立于生产解析器；包含三种类型与负整数。
func TestXSettingsTypedSnapshot(t *testing.T) {
	for _, order := range []string{"little", "big"} {
		t.Run(order, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "xsettings-"+order+".bin"))
			if err != nil {
				t.Fatal(err)
			}
			values, err := parseXSettings(data)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]xSetting{
				"Net/ThemeName": {kind: 1, text: "Adwaita-dark"}, "Gtk/FontName": {kind: 1, text: "Serif 15"},
				"Gtk/EnableAnimations": {kind: 0, integer: -7}, "Example/Color": {kind: 2, color: [4]uint16{0x1122, 0x3344, 0x5566, 0x7788}},
			}
			if len(values) != len(want) {
				t.Fatal("字段数量不符", values)
			}
			for name, value := range want {
				if values[name] != value {
					t.Fatalf("%s=%+v，预期%+v", name, values[name], value)
				}
			}
			// 每个截断点均须拒绝整份快照，不能留下前面已经解析的合法字段。
			for i := range data {
				v, err := parseXSettings(data[:i])
				if err == nil || v != nil {
					t.Fatalf("接受截断或发布部分数据：%d", i)
				}
			}
			if v, err := parseXSettings(append(data, 0)); err == nil || v != nil {
				t.Fatal("接受声明数量之外的数据")
			}
		})
	}
}

func TestXSettingsRejectsMalformedEntries(t *testing.T) {
	valid, err := os.ReadFile("testdata/xsettings-little.bin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"非法字节序", func(b []byte) []byte { b[0] = 2; return b }},
		{"非法类型", func(b []byte) []byte { b[12] = 3; return b }},
		{"非法字段名", func(b []byte) []byte { b[16] = '/'; return b }},
		{"超大声明长度", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[36:40], ^uint32(0)); return b }},
		{"超大数量", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:12], ^uint32(0)); return b }},
		{"重复字段", func(b []byte) []byte {
			one := append([]byte(nil), b[12:52]...)
			b = append(b[:52], one...)
			binary.LittleEndian.PutUint32(b[8:12], 2)
			return b
		}},
		{"超出原生读取上限", func(b []byte) []byte { return append(b, make([]byte, 1<<20)...) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if v, err := parseXSettings(tc.edit(append([]byte(nil), valid...))); err == nil || v != nil {
				t.Fatal("接受无效快照", v)
			}
		})
	}
	for _, name := range []string{"", "/A", "A/", "A//B", "1A", "A/1B", "A.B", "A-Name", "A\x00B", "A/中文"} {
		if validXSettingName([]byte(name)) {
			t.Error("接受非法名称", name)
		}
	}
	for _, name := range []string{"A", "_", "A_1/B2", "Net/ThemeName"} {
		if !validXSettingName([]byte(name)) {
			t.Error("拒绝合法名称", name)
		}
	}
	empty := make([]byte, 12)
	if v, err := parseXSettings(empty); err != nil || len(v) != 0 {
		t.Fatal("空快照不是合法值", err)
	}
}

func FuzzXSettingsSnapshot(f *testing.F) {
	for _, order := range []string{"little", "big"} {
		b, err := os.ReadFile("testdata/xsettings-" + order + ".bin")
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		values, err := parseXSettings(data)
		if err != nil && values != nil {
			t.Fatal("失败后返回部分快照")
		}
	})
}
