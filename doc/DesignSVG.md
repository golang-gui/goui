# 静态 SVG Icon 适配

## 范围与分层

`icon/svg` 是可选资源适配包，直接实现公开的 gui.IconSource.Image 接口。
gui/ui 不反向依赖解析器；platform 不认识 SVG、主题或控件。没有图标名称注册、
系统主题搜索、字体图标或全局资源管理器。

```go
source, err := svg.Parse(strings.NewReader(svgText))
if err != nil {
    return err
}
// 在 UI build 外解析；source 可供多个控件共享。
original := source.WithRawColors()
ui.Icon(source).Size(20).Style(modern.AccentIcon)
ui.Icon(original).Size(20)
```

入口为 `Parse(io.Reader) (*Source, error)`，返回值直接提供 Image 和 WithRawColors。
失败返回 nil 与 error。文件、embed 和网络加载
由应用负责；解析器不访问外部资源。返回值默认按完整图像 alpha 遮罩重新着色，
不是把 SVG 中某一个颜色替换成主题色。WithRawColors 保留原始 RGBA。
currentColor 在本 profile 中固定解析为黑色（默认模式只关心覆盖度），不实现 CSS
color 继承；原色模式也遵守此约定。

## 首版支持范围

这是明确受限的静态图标 profile，不承诺浏览器 SVG 兼容性。

- 单个 svg 根，有限且正的 viewBox，或 width/height；非零 viewBox 原点。
- 按比例居中 contain（xMidYMid meet），透明补边，不新增 Icon Fit 属性。
- path、rect、circle、ellipse、line、polyline、polygon、嵌套 g。
- fill/stroke、nonzero/evenodd 填充、线宽、端点、连接、miterlimit、dash/offset。
- 固定颜色、fill-opacity/stroke-opacity，祖先继承及局部覆盖；简单 inline style。
- translate、scale、rotate、matrix；描边局部变换仅支持等比伸缩、旋转和反射。
- defs 中直接定义的 linearGradient/radialGradient、stop、objectBoundingBox。
  渐变必须先定义后引用，不支持带局部变换的渐变图形。
- title/desc、常见 aria/data 元数据不转化为 Widget 语义。

明确拒绝：text、image、use、clip/mask/filter、script/animation、外部引用、CSS
样式表/选择器、非 1 的整体 opacity（需要离屏组透明合成）、gradientTransform、
userSpaceOnUse、非均匀/剪切描边变换、嵌套 svg、其他 preserveAspectRatio 模式。
不完整支持的内容不能靠日志告警后继续绘制；Parse 返回错误及 nil Source。

XML 上限 4 MiB、嵌套深度 64；栅格输出单边最多 4096 像素。这些是图标资源限制，
不是完整不可信 SVG 沙箱。该包用于应用选定的静态资产，不适合任意网络文档。
未承诺 SVG 全规范的非法值恢复和 CSS 长度单位；资源宜使用 viewBox 内的数值坐标。

## 实现与依赖证据

采用 `github.com/golang-gui/oksvg v0.0.0-20260918033125-75e7ce872d46`
解析几何与绘制样式。该 fork 基于 `fyne-io/oksvg v0.2.0`，复用仓库已有 rasterx
和 scanFT 栅格化。该依赖使 x/image 升至 v0.24.0，并引入对应 x/net/x/text 版本；
已将文字、图形包包含在全仓回归中。

分成三个步骤，而不是重写 Path 解析器：

1. normalize：检查支持范围，规范化继承、不完整的变换及描边处理。
2. compile：使用 oksvg 解析每个图形及祖先样式/共享 defs，保存不可变路径。
3. rasterize：创建当前尺寸 scanner，复制路径绘制状态，生成预乘 image.RGBA。

固定版本的 [oksvg 源码](https://github.com/fyne-io/oksvg/tree/v0.2.0) 有几处影响
本接口承诺的行为，均由包内像素回归约束：

- 路径坐标随变换变化，但描边宽度、dash/offset 需同步缩放。
- SetTarget 的组合不能直接满足非零 viewBox 原点的映射；使用
  T(居中) × S(等比缩放) × T(-viewBox 原点)，不修改共享 SvgIcon。
- 解析器未应用 fill-rule。按图形编译后设置公开的 UseNonZeroWinding，避免靠
  路径序号猜测（空图形不一定产生路径）、修改全局默认或访问私有字段。
- 填充/描边 opacity 的局部覆盖需在适配层归一；group opacity 不冒充它们。

这部分编译只发生在 Parse 时；共享 definitions 会按图形重复编译，不作为大型
SVG 文档的高性能导入器。后续依赖升级若原生修复相应问题，应由回归测试指导简化。
使用 scanFT 保留复合路径的 winding 规则，不用忽略该规则的 scanner 代替。

设计参考 [Qt QSvgRenderer](https://doc.qt.io/qt-6/qsvgrenderer.html) 的资源/控件分离，
但不照搬 QObject 生命周期、共享可变 renderer 或完整 SVG API。GOUI 保持 Source
内容不可变，Source 负责最终像素和颜色策略，Icon 负责样式解析与原生缓存；
资源生命周期详见当前 [IconSource 与 Icon 实现](../gui/icon.go)。

## 缓存、错误与并发

SVG 文档不持有 graphics.Image 或 Painter。每次栅格化只使用局部 scanner 和路径
状态；Source 自持一个有锁的最近尺寸原图/前景色结果缓存，返回的旧图片不再修改。
尺寸不同、颜色模式不同的 Icon 互不污染。包内并发测试同时渲染共享文档及调用
公开 Image，检查像素一致性并通过 race 检查。

Parse 返回加载/格式/不支持错误，并将第三方解析 panic 转换为错误。
Image(width, height, foreground) 不返回 error；资源已在 Parse 阶段准备完成。
请求超出每边 1..4096 像素或未初始化的 Source 返回 nil，由 Icon 使用缺失占位符；
有效的空 SVG 返回透明图片，不显示占位符。不将程序缺陷或 OOM 转换成加载错误。
Source 缓存原始像素，因此仅换前景色不重复解析或栅格化；Icon 不再二次着色。
原色模式忽略前景，但 Icon 无需识别此模式：Source 返回原有图片时可复用原生资源。
与 icon.Image 共用的着色算法位于 icon/internal/pixels；graphics 不提供图标着色工具。
WithRawColors 创建独立配置，请在 UI build 外创建并复用资源以保留内部缓存。
UI 重建会重新取图，不比较 Source；返回相同不可变图片时，Icon 复用已有原生上传资源。

## 验证记录（2026-09-17）

- `go test ./...`：通过，包含此前 ApplicationIdentity 回归。
- `go test -race ./gui ./ui ./icon/svg ./core/geometry ./platform/graphics/internal/textbitmap`：通过。
- Render 公开接口迁移新增 `icon/draw` 离屏适配、位图选择、任意最终颜色及公开 Source
  并发回归；`go test -race ./gui ./ui ./icon/... ./platform/graphics -count=1` 通过。
- `go test ./icon/svg -run '^$' -fuzz FuzzParse -fuzztime=5s -parallel=2`：通过，
  本轮 56,676 次输入执行；不代表不可信内容的安全审计。
- gui/ui/icon/svg/icon/draw 测试目标和 `tests/window/ui/icon`、`tests/window/ui/svgicon`：Windows/amd64、macOS/arm64
  使用 CGO_ENABLED=0 交叉编译通过。
- 像素断言覆盖非零原点、比例补边、复合孔洞、描边/虚线缩放、alpha、渐变及共享文档。
- 三平台真实窗口/后端外观操作尚未验收。入口 `tests/window/ui/svgicon` 的注释列出
  步骤、预期和环境记录要求；不依赖 DevServer。既有全仓测试仍可能连接桌面和生成
  源码目录内图片，本次不迁移那些旧测试；新增测试没有此类输出。

后续按实际资产需求补齐更复杂的 SVG 特性，不把本轮提升为完整 SVG 承诺。

## Tint 优化与实际图标验证（2026-09-18）

不改变先生成完整覆盖度、再按前景色着色的语义。RGBA/NRGBA 使用直接读 alpha
的快速路径，消除逐像素 image.Image.At 返回 color.Color 的分配；其他格式保留
通用实现。没有更换 SVG 栅格器，也不新增绘制后端 API。

包内测试覆盖所有 8 位 alpha、非零/负原点、SubImage/额外 Stride、半透明预乘色、
空图、原图与旧输出不变；保留原算法作为逐字节对照，并以独立公式核对结果。
RGBA64、Alpha、Paletted 与自定义包装类型验证通用路径。分配断言要求快速路径
最多两次分配（图片头和像素缓冲区），不随像素数量增加。

基准：Linux/amd64、Go 1.25.8、Xeon Gold 5318Y，单 CPU，每轮 150ms，三轮中位数。
同一输入与前景色，比较保存于测试内的原算法和当前 Tint，含结果分配、不含解析或上传：

| RGBA 尺寸 | 原算法 μs/op | 优化后 μs/op | 原分配次数 | 优化后分配次数 |
| --- | ---: | ---: | ---: | ---: |
| 24×24 | 20.24 | 3.74 | 578 | 2 |
| 48×48 | 82.36 | 14.17 | 2306 | 2 |
| 96×96 | 320.93 | 56.57 | 9218 | 2 |

重跑：`go test ./icon/internal/pixels -run '^$' -bench '^BenchmarkTint$' -benchmem -benchtime=150ms -count=3 -cpu=1`。
NRGBA 48×48 为 80.86 → 14.11 μs/op，分配同样为 2306 → 2。
这是换色 CPU 路径的收益，不表示稳定帧也按该比例加速。

窗口入口保留 brainSvg、peopleSvg、devToolSvg 原文，在 UI build 外解析并复用；
三行同时展示主题色、正文色、原色，另保留孔洞、描边、渐变、缺失占位与取图计数。
内容可滚动；第二窗口共享这三个 Source，独立拥有原生图片。

验证结果：

- `go test ./...`、`go test -race ./icon/... -count=1` 通过；全仓包括缓存结果和按既有开关跳过的桌面测试。
- icon 子包和窗口入口的 Windows/amd64、macOS/arm64 交叉编译通过（CGO_ENABLED=0、-exec=true）；未执行对应平台窗口验收。
- 复跑三个实际 SVG 的临时离屏对照：16～128 物理像素、多前景色/半透明，优化后与原 Tint 逐字节相同；与直接路径着色最大通道差异 1/255。
- Linux/XFCE，Software 1x、OpenGL 2x：窗口显示、亮暗/主题色/尺寸切换正常；Software 验证第二窗口打开/关闭后主窗口可继续操作。全部测试窗口已关闭。
- **形状验收未完全通过**：两个后端均发现 People 头部轮廓缺失，原色列同样错误。旧栅格器问题不属于本轮 Tint 修改；几何正确性不能由两种着色结果相同来证明。

### 连续弧线参数解析修复（2026-09-18）

oksvg v0.2.0 `path_cursor.go` 的 a/A 分支按参数段调用 AddArcFromA(points)，
但 AddArcFromA 内部调用 rasterx.AddArc 时传的是 c.points 而非当前 points，
导致同一命令内后续弧线使用了首段参数。
临时最小复现，在 20×20 viewBox、80×80 原色栅格输出中，以下等价路径有 944 个
alpha 像素不同（未经过 Tint）：

```text
M10 4a6 6 0 1 1 0 12 6 6 0 0 1 0-12
M10 4a6 6 0 1 1 0 12a6 6 0 0 1 0-12
```

已在自有 fork 修复：`rasterx.AddArc(points, ...)` 使用当前参数组，不改写输入 SVG。
后续独立对照确认 Brain 和 People 均受该缺陷影响，并非仅 People。

- 修复提交：[`8628585`](https://github.com/golang-gui/oksvg/commit/862858551be7055b63b24e02432db24b27b0d4e8)。
- 上游 PR：[`fyne-io/oksvg#6`](https://github.com/fyne-io/oksvg/pull/6)，只包含修复及回归测试。
- fork 的 master 另有模块路径提交 `75e7ce8`；GOUI 直接依赖 golang-gui 路径，
  不使用只对主模块有效、不会传递给下游应用的 replace，也不依赖本地绝对路径。
- 后续上游合并并发布稳定版本后，再评估迁回；本次没有改用 srwiley 版本。

fork 测试覆盖相对/绝对命令、多组不同半径/旋转/标志、后续相对直线及独立终点断言。
圆形覆盖测试以解析几何给出预期，排除距数学边界两物理像素内的抗锯齿区域，其他
位置精确断言 alpha。GOUI 的 `icon/svg/arc_test.go` 通过公开 Image 接口验证
40/80 物理像素、原色/半透明主题色；断言预乘 sRGB 通道，不依赖另一条解析路径。
两处新增回归均已确认旧依赖失败、修复后通过。

#### 独立 SVG 对照

Linux/amd64、Go 1.25.8，参考为 librsvg 2.52.5/Cairo，不用 oksvg 自己生成正确答案。
原始三个示例在 240×240 物理像素直接渲染原色，不经过 Tint。参考图中 3×3 邻域
alpha 全为 0 或全为 255 的位置作为内部采样点，统计 alpha 差大于 128 的严重形状
差异；此指标用于定位缺失轮廓，不等于所有抗锯齿像素一致。

| 原始示例 | 采样数 | 修复前差异 | 修复后差异 |
| --- | ---: | ---: | ---: |
| brainSvg | 49024 | 3075 | 0 |
| peopleSvg | 50485 | 5391 | 0 |
| devToolSvg | 50870 | 0 | 0 |

网上另取六个未改写 SVG，用 fork 原生 ScannerGV 在 256×256 渲染，与同尺寸
librsvg 对照。固定来源版本，下载资源和 PNG 位于仓库外临时目录，不引入运行依赖：

- [Fluent System Icons 1.1.252](https://github.com/microsoft/fluentui-system-icons/tree/5b07d607e90dc0226b658ed050e07944b2bef5b2)，
  [MIT 许可证](https://github.com/microsoft/fluentui-system-icons/blob/5b07d607e90dc0226b658ed050e07944b2bef5b2/LICENSE)：
  `assets/People/SVG/ic_fluent_people_24_regular.svg`、
  `assets/Globe/SVG/ic_fluent_globe_24_regular.svg`、
  `assets/Developer Board/SVG/ic_fluent_developer_board_24_regular.svg`。
  这个版本的 People 与窗口示例中省略重复命令的版本不同，用于未受影响路径回归。
- [Noto Emoji v2.047](https://github.com/googlefonts/noto-emoji/tree/22e564626297b4df0a40570ad81d6c05cc7c38bd)，
  [SVG Apache 2.0 许可证](https://github.com/googlefonts/noto-emoji/blob/22e564626297b4df0a40570ad81d6c05cc7c38bd/svg/LICENSE)：
  `svg/emoji_u1f409.svg`（dragon，25 条路径）、`svg/emoji_u1f3a0.svg`
  （carousel horse，31 条路径）、`svg/emoji_u1f680.svg`（rocket，25 条路径）。

六个素材在本次修复前后逐像素一致；与 librsvg 对照的内部严重 alpha 差异均为 0。
对于参考图中 3×3 邻域预乘 RGBA 完全相同的平坦区域，按 2/255 的通道舍入容差统计：
Fluent 三图和 rocket 没有超差；carousel 有 300/52183 点、dragon 有 62/49389 点超差。
carousel 的鼻部存在可见颜色/渐变差异，修复前即存在，不能归为抗锯齿差异或声称
这些素材全部正确。Noto 中的 userSpaceOnUse 渐变等也超出 GOUI 静态图标 profile；
这里验证的是底层 fork，不扩大 svg.Parse 的支持承诺。复杂渐变兼容性留待独立任务。

本轮几何验证是离屏验证；不替代 Windows/macOS 原生窗口、Direct2D 及跨屏 DPI 验收。

#### 本轮测试与交付状态

- fork 的修复分支与独立模块路径的 master 均已推送；两种模块路径下
  `go test ./... -count=1`、`go test -race ./... -count=1` 均通过。
- GOUI 的 `go test ./...` 和禁用缓存复跑 `go test -json -count=1 ./...` 通过；
  后者有 31 个测试/子测试按既有桌面验证开关跳过，不计为功能通过。
- `go test -race ./icon/... -count=1` 通过。
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -exec=true ./icon/... ./tests/window/ui/svgicon`
  与 `GOOS=darwin GOARCH=arm64` 对应命令均通过；仅交叉编译，没有执行平台窗口验证。
- 首次 `go mod download` 因 goproxy.cn 对新提交的校验查询返回 404 而失败，继而
  导致测试 setup 报 missing go.sum。用单次命令环境
  `GOPROXY=https://proxy.golang.org,direct GOSUMDB=sum.golang.org` 下载后重跑通过；
  没有关闭校验或修改用户的全局 Go 设置。最终依赖为远程固定伪版本，无 replace。
- fork 既有测试在其 testdata 生成被忽略的 PNG；GOUI 既有测试仍有桌面连接和源码
  目录图片输出。本次新增的回归测试只在内存绘制，不增加这些副作用。
- 本轮未重新操作真实窗口；Windows/macOS 原生运行、三后端图标窗口体验继续待验收。
