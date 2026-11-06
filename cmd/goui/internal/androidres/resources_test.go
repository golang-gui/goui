package androidres

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func u16(b []byte, p int) uint16 { return binary.LittleEndian.Uint16(b[p:]) }
func u32(b []byte, p int) uint32 { return binary.LittleEndian.Uint32(b[p:]) }

func readPool(t *testing.T, b []byte) []string {
	t.Helper()
	if u16(b, 0) != 1 || u32(b, 16) != 0 {
		t.Fatal("不是预期的 UTF-16 字符串池")
	}
	var result []string
	for i := 0; i < int(u32(b, 8)); i++ {
		p := int(u32(b, 20) + u32(b, 28+4*i))
		n := uint32(u16(b, p))
		p += 2
		if n&0x8000 != 0 {
			n = (n&0x7fff)<<16 | uint32(u16(b, p))
			p += 2
		}
		units := make([]uint16, n)
		for j := range units {
			units[j] = u16(b, p+2*j)
		}
		if u16(b, p+2*int(n)) != 0 {
			t.Fatal("缺少字符串终止符")
		}
		result = append(result, string(utf16.Decode(units)))
	}
	return result
}

func TestManifest(t *testing.T) {
	input := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="org.example.App" android:versionCode="42"><uses-sdk android:minSdkVersion="29" android:targetSdkVersion="35"/><application android:label="中文 🐹 &amp; العربية" android:debuggable="false" android:icon="@mipmap/app_icon" android:theme="@android:style/Theme.Material.Light.NoActionBar"/></manifest>`
	b, err := Manifest([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if u16(b, 0) != 3 || int(u32(b, 4)) != len(b) {
		t.Fatal("XML 总长度不正确")
	}
	strings := readPool(t, b[8:])
	seen := map[string]bool{}
	for p := 8; p < len(b); p += int(u32(b, p+4)) {
		if u16(b, p) != 0x102 {
			continue
		}
		for i := 0; i < int(u16(b, p+28)); i++ {
			a := p + 36 + 20*i
			name, data := strings[u32(b, a+4)], u32(b, a+16)
			seen[name] = true
			switch name {
			case "minSdkVersion":
				if b[a+15] != 16 || data != 29 {
					t.Fatal("最低 API 类型或值错误")
				}
			case "targetSdkVersion":
				if data != 35 {
					t.Fatal("目标 API 错误")
				}
			case "versionCode":
				if data != 42 {
					t.Fatal("版本号错误")
				}
			case "debuggable":
				if b[a+15] != 18 || data != 0 {
					t.Fatal("布尔值编码错误")
				}
			case "label":
				if b[a+15] != 3 || strings[data] != "中文 🐹 & العربية" {
					t.Fatal("多语言标签被破坏")
				}
			case "icon":
				if b[a+15] != 1 || data != 0x7f010000 {
					t.Fatal("图标资源 ID 错误")
				}
			case "theme":
				if data != 0x01030241 {
					t.Fatal("系统主题 ID 错误")
				}
			}
		}
	}
	for _, name := range []string{"package", "minSdkVersion", "targetSdkVersion", "versionCode", "debuggable", "label", "icon", "theme"} {
		if !seen[name] {
			t.Fatalf("缺少属性 %s", name)
		}
	}
	for _, invalid := range []string{"", "<application/>", "<manifest/><manifest/>", "<manifest>", `<manifest xmlns:android="http://schemas.android.com/apk/res/android" android:unknown="x"/>`, `<manifest xmlns:android="http://schemas.android.com/apk/res/android" android:icon="@other/icon"/>`} {
		if _, err := Manifest([]byte(invalid)); err == nil {
			t.Fatalf("未拒绝无效输入 %q", invalid)
		}
	}
}

func TestResources(t *testing.T) {
	b, err := Resources("org.example.App")
	if err != nil {
		t.Fatal(err)
	}
	if u16(b, 0) != 2 || int(u32(b, 4)) != len(b) || u32(b, 8) != 1 {
		t.Fatal("资源表头错误")
	}
	paths := readPool(t, b[12:])
	p := 12 + int(u32(b, 16))
	if u16(b, p) != 0x200 || u32(b, p+8) != 0x7f {
		t.Fatal("资源包 ID 错误")
	}
	var packageName []uint16
	for i := 0; i < 128 && u16(b, p+12+i*2) != 0; i++ {
		packageName = append(packageName, u16(b, p+12+i*2))
	}
	if string(utf16.Decode(packageName)) != "org.example.App" {
		t.Fatal("资源包名错误")
	}
	seen := map[uint16]bool{}
	for q := p + int(u16(b, p+2)); q < len(b); q += int(u32(b, q+4)) {
		if u16(b, q) != 0x201 {
			continue
		}
		dpi := u16(b, q+34)
		if seen[dpi] {
			t.Fatal("重复密度配置")
		}
		seen[dpi] = true
		if u32(b, q+12) != 1 || u32(b, q+16) != 88 || u16(b, q+88) != 8 || b[q+99] != 3 {
			t.Fatal("图标条目布局错误")
		}
		index := u32(b, q+100)
		want := map[uint16]string{160: "mdpi", 240: "hdpi", 320: "xhdpi", 480: "xxhdpi", 640: "xxxhdpi"}[dpi]
		if paths[index] != "res/mipmap-"+want+"/app_icon.png" {
			t.Fatal("密度与图标路径不一致")
		}
	}
	if len(seen) != 5 {
		t.Fatal("必须包含五种密度")
	}
	if _, err := Resources(strings.Repeat("x", 128)); err == nil {
		t.Fatal("必须拒绝超长资源包名")
	}
	var p2 pool
	text := strings.Repeat("中🐹", 16384)
	p2.add(text)
	if readPool(t, p2.encode())[0] != text {
		t.Fatal("长字符串或代理对编码错误")
	}
}
