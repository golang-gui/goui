# GOUI v0.7.0 发布说明

[v0.7.0 已正式发布](https://github.com/golang-gui/goui/releases/tag/v0.7.0)，三平台 Demo 为 **0.7.0 / build 1**。
标签及正式附件冻结于提交 `33ce8a9e02f57364f2f18aaed1b36d4ca32ef127`；发布后的文档收尾不移动标签。
本版本以 v0.6.0 为基线，来源、摘要与完整验证范围见 Release 的
[BUILD-INFO.md](https://github.com/golang-gui/goui/releases/download/v0.7.0/BUILD-INFO.md) 和
[SHA256SUMS](https://github.com/golang-gui/goui/releases/download/v0.7.0/SHA256SUMS)。

## 主要变化

- 共享 Command／CommandGroup，菜单与快捷键复用操作，打开的菜单实时更新可用性；
  快捷键提示独立对齐。UI 支持窗口默认命令组、DefineCommand、BindEnabled 和外部查询。
- 四个独立圆角与匹配 BoxShadow，BorderSides 选择单边、平行边、L形或三边，保持
  统一宽度／颜色。修正 OpenGL 收尖、半透明路径覆盖和连排按钮接缝。
- 裁剪外子树跳过绘制，忙等待指示器和示例动画不可见时停止续帧；可选 FPS HUD
  闲置为0，Go runtime/trace 记录帧任务及阶段，均为 CPU／运行时观测。
- 优化本地化缓存，修正表格末端网格、装饰范围、可变行高滚动及菜单活动项清理。
- Android／iOS 平台、文字、Software／OpenGL ES 及实验性构建能力继续推进；
  完整移动端 GUI、触摸、IME 和物理设备交付仍在开发，本次不附移动安装包。
- Demo 图形页展示独立圆角、阴影和四组选边框，菜单页展示共享操作，保留八语言与主题切换。

## 使用与 API 迁移

```sh
go get github.com/golang-gui/goui@v0.7.0
```

- `Style.Radius()` 改为 `Style.CornerRadius()`，读取完整 `geometry.CornerRadius`；
  构造规则的 `Rule.Radius(float32)` 继续可用。
- `Painter.DrawBoxShadow` 的 radius 参数改为 `geometry.CornerRadius`；统一半径使用
  `geometry.UniformCornerRadius(r)`。
- `Path.QuadBezierTo` 改用 `platform/graphics/utils.QuadBezierTo`；轮廓生成工具同在该工具包。

## Demo 与验证

下载 Windows amd64 EXE、Linux amd64/X11 可执行文件或 macOS arm64 ZIP。
Linux 执行 `chmod +x goui-demo-v0.7.0-linux-amd64`；macOS 解压打开 `GOUI Demo.app`。
Windows EXE 未做 Authenticode 签名。Mac 使用 Developer ID 签名、Hardened Runtime 与
安全时间戳；Apple 公证任务 `9e240d1c-d212-47ab-8d6a-4129e52a2eb1` 为 Accepted。
最终 ZIP 解压后再次通过签名、票据、Gatekeeper 和实际窗口验收。

发布构建使用 Go1.25.8、CGO_ENABLED=0、trimpath、`-s -w` 和同源 cmd/goui。
冻结源码在独立 Xvnc／Xfwm4 会话的全仓非缓存测试为 **4508通过／11跳过／0失败**；
相关 race／vet 通过。Linux OpenGL／Software、Windows10 Direct2D、macOS15.7.7
默认 OpenGL 的1x最终程序完成对应桌面操作，具体覆盖差别以 BUILD-INFO 为准。
Go1.24.13 的独立消费项目从公开代理获取本标签，无 replace／workspace，三个桌面目标编译通过。

桌面 `go build ./...` 仍受两个移动原生验收目录缺少桌面 main 阻断，Go1.24／1.25
均能复现；排除它们后，三端库、工具与示例构建通过，未静默修改冻结代码。
Windows OpenGL、真实混合 DPI／Retina、高刷新呈现、全量 IME／DnD 和物理移动
设备未在本轮重新验收。README 下载链接已更新，现有截图仍明确标注为 v0.6.0。

Draft 的五份附件核对服务端 digest 并重新下载，与本地原件逐字节一致；公开发布后
再以匿名下载全部附件，SHA256SUMS 和原件比较全部通过。正式版本已设为 Latest。

## English summary

- Add shared commands, live menu availability and aligned shortcut columns.
- Add independent corners, matching shadows and selectable border edges; correct sharp-tip coverage and seams.
- Add clip-based paint culling, drawable-aware frame requests, an idle-aware FPS HUD and Go execution tracing.
- Improve localization caching, table boundaries/scrolling and menu activity cleanup; advance experimental mobile foundations.
- Update Demo to 0.7.0 / build 1. Windows amd64, Linux amd64/X11 and notarized macOS arm64 assets come from one frozen commit.
- Go 1.24 remains the declared minimum; see BUILD-INFO.md for exact verification and known build exceptions.

Full changes: https://github.com/golang-gui/goui/compare/v0.6.0...v0.7.0
