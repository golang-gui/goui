# TextView 尺寸与单行外观验证

2026-10-19；GOUI 基线 83c830e；SourceView 联调基线 c8cf6c9。
设计见 [尺寸契约](DesignTextViewSizing.md)。本轮未修改 platform 或新增公共接口。

## 实施

- TextView 使用原有 Measure，在当前宽度下测量内容高度和首行基线。
  空行、末尾 LF、软换行及预编辑参与显示测量；Placeholder 不参与尺寸。
- 两种文本控件共享段落基线、行高及回退字体容纳规则，绘制/命中/选区/IME/锚点
  使用一致位移；原生指标缓存与按需构建的编辑 Geometry 分开。
- ScrollView 使用子项 Measure 申请高度，完整范围仍来自 Scrollable.ContentSize。
  不增加额外测量协议、自然高度模式、开关或高度信号；复用 MaxSize/MainWeight。
- ListView 使用其已维护的估计内容范围报告尺寸，不在 Measure 中创建全部行。
  TableView 对有界表体显式使用紧高度，保留现有填充行为。
- 横向容器需要填满交叉轴时显式 CrossStretch。相关 GUI/UI 测试保留原始最终
  矩形断言，增加约束表达；尺寸修复同步更新原生指标缓存的模拟失效夹具。
- Basic/Modern 的多行输入外观与单行输入保持一致，保留独立样式名称。

## 本机结果

环境：Linux amd64，Go 1.25.8，Pango，TigerVNC 1.12.0 / Xfwm4 4.16.1。
窗口使用临时 :145 显示连接，与日常桌面隔离；无剪贴板或系统设置修改。

通过：

- 修改前 `go test ./...` 基线；修改后 `go test ./...` 全仓。
- `go test -race ./gui ./ui ./widgets ./theme/...`。
- `tests/window/gui/text_sizing`：Software/OpenGL 各 1x、2x，九步断言，
  每组包含 Basic、Modern Light/Dark。通过原生 Painter image 类型和 PixelScale
  核对实际后端与缩放；同一后端的单行及 Placeholder RGBA 逐像素相同，无容差。
  正常输入事件验证增高、96 DIP 上限、滚动范围、删除收缩、软换行和宽度变化；
  包内独立 10x20 DIP 字体夹具覆盖首行基线、命中/光标、预编辑及缓存边界。
- 既有 `tests/window/gui/textplaceholder`，Software 1x。
- SourceView FindBar 外观窗口：Software 1x/2x；OpenGL 1x、18 pt 字号。
  包括两行独立增长、200 DIP 输入上限、收缩归零及原有样式/宽度/焦点断言。
- SourceView 查找、正则/范围/捕获组窗口，Software 1x。
- `GOOS=windows GOARCH=amd64 go build ./...`、
  `GOOS=darwin GOARCH=arm64 go build ./...`（两仓，SourceView 使用本地工作区）。

未执行：Windows/macOS 原生窗口、真实物理 IME/候选窗、多平台桌面合成。
交叉构建不是这些场景的运行验收。没有对时延或内存作前后性能结论；无界内容
测量需要遍历完整内容，有界测量保留提前停止和原有段落缓存预算。

首次回归中发现的旧视口高度约定、TableView 有界表体分配和测试夹具原生指标
失效问题均已修正；最终命令通过。临时日志、截图和二进制均在仓库外。
