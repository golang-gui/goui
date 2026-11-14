//go:build ignore

// 生成窗口验收辅助脚本；输出由GOUI_GENERATE_DIR限定在源码仓库之外。
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/gui/drag_preview", map[string]string{
		"inject.py": `#!/usr/bin/env python3
"""创建只用于窗口验收的Go源码overlay，不修改生产源码。

先在GOUI仓库执行go generate ./tests/window/gui/drag_preview（设置GOUI_GENERATE_DIR）。
示例：python3 /生成目录/tests/window/gui/drag_preview/inject.py x11 draw /绝对路径/输出目录 --repo /GOUI仓库
随后go build -overlay=/绝对路径/输出目录/overlay.json ./tests/window/gui/drag_preview
故障点通过精确源码匹配定位；源码变化后必须人工复核，不静默跳过注入。
可用--source传入旧版本同名源码，验证旧实现确实不能完成同一次拖放。
"""
import argparse
import json
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("backend", choices=["x11", "win32", "cocoa"])
parser.add_argument("fault")
parser.add_argument("output", type=Path)
parser.add_argument("--source", type=Path)
parser.add_argument("--repo", type=Path, required=True, help="GOUI仓库根目录")
args = parser.parse_args()
root = args.repo.resolve()
if not (root / "go.mod").is_file():
    parser.error("--repo必须指向GOUI仓库根目录")
if args.output.resolve() == root or root in args.output.resolve().parents:
    parser.error("故障构建输出必须在GOUI仓库之外")
paths = {"x11": "platform/linux/x11/dragdrop_linux.go",
         "win32": "platform/windows/win32/dragdrop_windows.go",
         "cocoa": "platform/darwin/cocoa/dragdrop_darwin.go"}
target = root / paths[args.backend]
source = (args.source or target).read_text()


def replace(old, new):
    global source
    if source.count(old) != 1:
        raise RuntimeError(f"故障点必须唯一：{old!r}，找到{source.count(old)}处")
    source = source.replace(old, new, 1)


if args.backend == "x11":
    if args.fault == "create":
        replace("p.window, err = newNativeWindow(paint, true, raster.Rect.Dx(), raster.Rect.Dy(), true)",
                'fmt.Println("FAULT x11 create")\n p.window, err = nil, fmt.Errorf("injected preview window failure")')
    elif args.fault == "draw":
        replace("if err := p.window.drawImage(bitmap); err != nil {",
                '''if err := func() error {
                    if err := p.window.drawImage(bitmap); err != nil { return err }
                    fmt.Println("FAULT x11 draw after allocation")
                    return fmt.Errorf("injected preview draw failure")
                }(); err != nil {''')
        replace("p.window.Destroy()\n\t\treturn nil, err", '''wid := p.window.wid
                p.window.Destroy()
                if _, exists := windowMap[wid]; exists { panic("preview window leaked") }
                fmt.Println("CLEAN x11 preview window")
                return nil, err''')
    elif args.fault == "session":
        replace("p.display.GrabPointer(d.window.wid, false, mask, s.cursor(0), press.event.Time)",
                'func() int { fmt.Println("FAULT x11 session grab"); return 1 }()')
        # 保留局部常量及光标创建路径，避免负对照变成编译失败。
        replace("const mask = xlib.EventMaskPointerMotion | xlib.EventMaskButtonRelease",
                "const mask = xlib.EventMaskPointerMotion | xlib.EventMaskButtonRelease\n _ = mask\n _ = s.cursor(0)")
    else:
        parser.error("x11支持create、draw、session")
elif args.backend == "win32":
    if args.fault == "bitmap":
        replace("bitmap, err := winapi.CreateDIBSection(&bmi, &bits)", '''bitmap, err := func() (winapi.HBITMAP, error) {
            _ = bmi
            fmt.Println("FAULT win32 bitmap")
            return 0, fmt.Errorf("injected DIB failure")
        }()''')
    elif args.fault == "helper":
        replace("helper, hr := shell.NewDragSourceHelper()", '''fmt.Println("FAULT win32 helper after bitmap")
            helper, hr := (*shell.DragSourceHelper)(nil), com.HRESULT(-2147467259)''')
    elif args.fault == "partial":
        replace("hr = helper.InitializeFromBitmap(&native, unsafe.Pointer(data))", '''hr = func() com.HRESULT {
            _ = native
            format := winapi.RegisterClipboardFormat("GOUI injected partial preview")
            memory := winapi.GlobalAlloc(winapi.GMEM_MOVEABLE | 0x0040, 4)
            if memory == 0 { panic("cannot allocate injected medium") }
            medium := com.StgMedium{Tymed: com.TymedHGlobal, Handle: uintptr(memory)}
            query := com.FormatEtc{Format: format, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal}
            // 真实COM SetData接管该介质，并将辅助格式写入预览对象。
            result := com.HRESULT(int32(oleDataSetData(uintptr(unsafe.Pointer(data)), uintptr(unsafe.Pointer(&query)), uintptr(unsafe.Pointer(&medium)), 1)))
            if result.Failed() { panic("injected SetData failed") }
            injectedPreviewData = data
            fmt.Println("FAULT win32 partial SetData")
            return com.HRESULT(-2147467259)
        }()''')
        source += "\nvar injectedPreviewData *oleDataObject\n"
        # 旧实现没有此helper；仍可构建，原生启动错误构成独立负对照。
        if "func newOLESourceData(" in source:
            old = "return newOLEDataObject(wire)"
            source = source.replace(old, '''return func() *oleDataObject {
                format := winapi.RegisterClipboardFormat("GOUI injected partial preview")
                if _, exists := wire[format]; exists { panic("partial preview polluted portable payload") }
                if injectedPreviewData != nil {
                    if _, exists := oleSourceRoots.Load(uintptr(unsafe.Pointer(injectedPreviewData))); exists { panic("failed preview object still rooted") }
                    fmt.Println("CLEAN win32 partial object and formats")
                }
                return newOLEDataObject(wire)
            }()''')
    else:
        parser.error("win32支持bitmap、helper、partial")
else:
    if args.fault == "image":
        replace("native := NewNativeImage(NewNativeData(encoded.Bytes()))", '''fmt.Println("FAULT cocoa native image decode")
            native := NewNativeImage(NewNativeData([]byte("injected invalid image")))''')
    elif args.fault == "encode":
        replace("if err := png.Encode(&encoded, img); err != nil {", '''if err := func() error {
            if err := png.Encode(&encoded, img); err != nil { return err }
            fmt.Println("FAULT cocoa encoded preview")
            return fmt.Errorf("injected preview encoding failure")
        }(); err != nil {''')
    else:
        parser.error("cocoa支持image、encode")

args.output.mkdir(parents=True, exist_ok=True)
output = args.output.resolve() / target.name
output.write_text(source)
subprocess.run(["gofmt", "-w", str(output)], check=True)
overlay = args.output.resolve() / "overlay.json"
overlay.write_text(json.dumps({"Replace": {str(target): str(output)}}, indent=2) + "\n")
print(overlay)
`,
	})
}
