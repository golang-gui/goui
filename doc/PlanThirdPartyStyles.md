# 第三方控件样式注册验收（2026-10-17）

设计见 [DesignThirdPartyStyles](DesignThirdPartyStyles.md)。实现包含 Basic / Modern 各自的
Register、完整规则组合、初始化冻结、名称排序和 GUI 默认 Basic 的延迟缓存。
未修改平台后端、style 值模型、UI 协调机制或 SourceView 依赖方向。

## 已通过

- Linux amd64，Go 1.25.8：`go test ./...`，包括已有未跟踪测试；
  `go test -race ./theme/basic ./theme/modern`。
- 注册测试：非法/重复/过晚调用、顺序、只读内置 base、规则切片及 sheet 隔离；
  独立进程 fixture 验证 gui 先导入时的默认缓存和仅 Basic 控件在 Modern 下的结果。
- macOS 15.7.7 arm64，Go 1.24.9；Windows 10 19045 amd64，Go 1.24.0：
  `go test ./theme/basic ./theme/modern ./gui ./widgets ./ui -count=1`，使用本次暂存树归档。
- SourceView 联调的全包测试与 Linux Pango / Software 1x、2x / OpenGL 1x 窗口验证，
  包含同窗口两组亮暗 StyleScope、单侧换主题、18 pt 字体、补全及 FindBar 交互。

## 失败与验证边界

`go vet ./...` 仍失败，输出有 222 条诊断，涉及原生 unsafe.Pointer 与既有未命名字段字面量。
对照修改前 HEAD，补入原已存在的 `ui/text_editing_test.go` 和 `example/gui_test/main.go`
后，诊断逐条排序完全一致，无新增告警。这两份未跟踪文件未纳入提交。
本机测试不使用桌面；窗口联调在隔离的 TigerVNC / Xfwm4（compositor off）执行，退出后清理。

远端只验证原生包测试与程序构建，没有宣称完成 Windows/macOS 桌面外观、IME 或混合 DPI 验收。
工作区里的既有三份文档修改及其他未跟踪文件保持原状，不纳入该功能提交。
