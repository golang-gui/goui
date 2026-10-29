# 静态 SVG Icon 适配

## 范围与分层

`icon/svg` 是实现 `gui.IconSource` 的可选资源适配包。SVG 的 XML、命名空间、样式、
引用、路径、变换和栅格化统一由自维护 `github.com/golang-gui/oksvg v0.1.0` 负责。
GOUI 只负责输入/图片尺寸限制、原色或前景着色、CPU 图片缓存与控件接入。
不重写 SVG、不剥离属性、不按图形重复编译，也不在 GOUI 补偿底层绘制错误。

```go
source, err := svg.Parse(strings.NewReader(svgText))
if err != nil {
    return err
}
// Prepare sources outside UI build to preserve their image caches.
original := source.WithRawColors()
ui.Icon(source).Size(20).Style(modern.AccentIcon)
ui.Icon(original).Size(20)
```

保持 `Parse(io.Reader) (*Source, error)`、`WithRawColors` 和
`Image(width, height int, foreground gui.Color) image.Image`。不修改 IconSource、
gui/ui 或 Painter 的接口。文件、embed 及网络加载仍由应用完成。

## 解析与错误

Parse 最多读取 4 MiB，然后直接调用 `oksvg.ReadIconStream(..., StrictErrorMode)`。
不建立 GOUI 自有属性白名单；底层已经支持的内容不会在适配层再次拒绝。
DOCTYPE、xml:space、clip-rule、命名空间元数据交由解析器处理；不获取 DTD、
不展开自定义实体、不读取外部文件或 URL。未知属性的前向兼容与已知未实现绘制能力的
Strict 拒绝由同一个解析器决定。clip-rule 不代替 fill-rule，也不意味着支持 clipPath。

解析错误返回 nil Source 和带上下文的 error，不返回部分文档。不使用宽泛 recover
将程序缺陷转换为普通加载错误；可复现的解析器/栅格器缺陷在相应自维护仓库修复。
完整深度、引用和数值校验由固定版本的 oksvg 实现，不能据此承诺完整不可信 SVG 沙箱。

支持路径、基本形状、局部 use、简单 class/inline 样式、color/currentColor 继承、
填充规则、描边/虚线、百分比及绝对单位、仿射变换、线性/径向渐变、后置资源定义等。
支持范围以固定版本的 [oksvg 说明](https://github.com/golang-gui/oksvg/tree/v0.1.0)
为准，不扩展为浏览器 SVG 或完整 CSS 的承诺。文本、外部图像、filter、mask、实际 clipPath、
pattern、非 1 整体 opacity、嵌套 svg 等未实现绘制仍报错。
非零 fr 及边界焦点配合 repeat/reflect 的部分退化语义仍不是完整支持能力。

## 栅格化与颜色

Source 私有持有解析后的 SvgIcon，不向调用方暴露可变解析结构。
Image 对合法尺寸直接调用 `SvgIcon.Rasterize`，获得透明初始化的预乘 `image.RGBA`。
viewBox、非零原点、preserveAspectRatio、局部及视口描边缩放全部由该接口处理。
默认 xMidYMid meet；文档显式声明的 none、其他对齐和 slice 按文档生效。
GOUI 不再创建 SVG Scanner、遍历 SVGPaths 或重复缩放线宽、dash 和 offset。

默认模式按完整图像 alpha 遮罩和 Icon 的预乘前景色着色，复用 icon/internal/pixels.Tint。
这不等于把 SVG 内的 currentColor 替换为前景色。WithRawColors 保留文档 RGBA，忽略前景参数；
原色模式的 currentColor 遵循 SVG 自身 color 及其继承，无声明时为默认黑色。
项目 Logo 继续使用原色配置。

## 缓存与资源归属

Source 保留一个有锁的最近尺寸原图缓存和一个最近前景色的着色缓存。
相同尺寸/前景返回同一张不可变图片；仅更换前景不重复解析或栅格化；尺寸变化才重新栅格化。
旧返回图片不再修改。WithRawColors 创建共享解析结果、缓存互相独立的新配置。
底层 Rasterize 使用调用局部状态，多个配置可并发使用同一解析结果。

每边输出仍限定 1..4096 物理像素。nil/零值 Source 或非法尺寸返回 nil；
有效空 SVG 返回有尺寸的透明图片，不显示缺失占位。Image 不添加错误回调。
Source 不持有 graphics.Image、Painter 或 Window；gui.Icon 继续管理原生上传缓存和释放时机。

## 工具接入

`Source.AspectRatio() float64` 返回解析后 viewBox 宽/高，与请求图片尺寸无关；
nil 或未初始化的 Source 返回零。它只属于具体 svg.Source，不扩展 IconSource。
cmd/svgpng 用该只读值处理长边尺寸，移除其独立 XML/长度解析；显式 WxH 仍请求指定画布。
cmd/goui 继续通过相同 Source 生成 ICO/ICNS，不另建 SVG 解析或绘制路径。

## Software 范围

SVG 接入切片未迁移 Software Painter；后续独立切片将它也迁移到
golang-gui/rasterx、golang-gui/scanFT v0.1.0，移除 srwiley 两项依赖。
两条链路现在使用相同版本，但不共享可变 Scanner、Stroker 或目标缓冲区。
Software 的使用方契约与像素回归见 [绘制颜色设计](DesignGraphicsColor.md#software-栅格器依赖)。
SVG 仍先输出 CPU 位图，再按普通图片接入三个绘制后端，不借用 Software Painter 来解释 SVG。

## 测试与验收

包内保留原有缓存、孔洞、描边/虚线缩放、透明度、渐变、并发、尺寸限制和非法输入覆盖。
原来因适配层限制拒绝的合法 SVG 改为独立像素正向断言；不删除有效断言或扩大容差。
补充 DOCTYPE、命名空间、clip-rule、class/use、后置渐变、非均匀描边、preserveAspectRatio、
圆外焦点和近直线端部。固定 resvg 原始素材附来源及许可；测试不访问网络。

本机独立 X11 会话运行 tests/window/ui/svgicon：Software/OpenGL 的 1x/2x，
检查 Brain/People/Dev tool、亮暗/主题/尺寸、资源替换、多窗口共享及新增兼容性行。
真实操作经过正常平台事件；不以 DevServer 或仅打开窗口代替操作验收。
执行全仓 go test、相关 race/fuzz、Windows amd64/macOS arm64 编译；实际结果另记 Plan。

### v0.1.0 接入验收（2026-10-29）

同一公开 Parse/Image 最小例程确认：改动前 DOCTYPE、xml:space、clip-rule、
color/currentColor 四组被适配层拒绝，改动后均按独立预期输出蓝色内部像素。
新增固定素材和解析圆盘断言覆盖圆外焦点、近直线端部和连续弧线；不靠改写素材或
扩大容差通过。包内、全仓、race/fuzz、跨目标编译及本机独立 X11 四组窗口验收已完成，
环境、跳过项及异常记录见 [Plan](Plan.md#svg-原生栅格化接入2026-10-29)。
Windows/macOS 只完成交叉编译，未将其计为原生窗口验收。Software Painter 未修改。

## 历史验证记录

下列日期记录对应旧适配实现及当时限制；旧 normalize/逐图形编译与补偿不是当前方案。

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
