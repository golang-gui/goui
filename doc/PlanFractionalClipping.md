# 小数位置绘制裁剪

状态：小数裁剪与旋转圆弧的后续修复已完成；验证范围和独立质量缺口见下方。

## 根因与修复边界

- Linux amd64 / X11 / Xfwm4 / OpenGL 1x：48／64 DIP、4 DIP 环宽的圆环在
  整数位置正常，顶部位于半物理像素时被截平。OpenGL SetClipRect 先就近取整，
  Path 却保留小数坐标，因而将本应保留的边缘像素排除。Direct2D 原有同类取整，
  Software 已采用包络，但小尺寸、零面积和取消裁剪的区分不完整。
- 单独修改裁剪仍不足：NanoVG v0.0.13 为避免半透明 AA 边缘重复合成，会在
  fringe 绘制后写入模板值，却只在原路径 bounds 内清理。半像素边界上的模板
  残留会阻挡后续路径；即使取消 Scissor 也能复现。ProgressBar 的透明轨道和前景
  连续绘制可以触发该问题，不是圆环几何或额外布局留白的问题。
- NanoVG v0.0.14 的填充清理四边形包含实际生成的 fill／fringe 顶点，清理所有
  被写入的模板像素。后续 v0.0.15 保留该修复；GOUI 固定依赖正式标签，不含本地
  replace 或控件补偿。

## 裁剪契约

1. GUI 保留 DIP 逻辑交集。后端只对最终交集换算一次保守物理像素包络，左上
   floor、右下 ceil；不分别扩大祖先再求交，不改变任意路径坐标。
2. OpenGL 使用整数 Scissor，Direct2D 使用同一包络的 ALIASED clip，Software
   使用相同私有换算。保留图元自身 AA，避免再乘一次小数裁剪覆盖率；文字缓存的
   可见区域与图元使用一致包络。普通轴对齐图元仍按既有就近取整规则绘制。
3. 包络外像素不能改变。相邻小数逻辑区域可以共享边界像素，这是硬矩形裁剪，
   不是精确亚像素蒙版或圆角子树裁剪。逻辑空交集始终跳过，不因包络重新出现。
4. 仅全零 Rectangle{} 取消平台裁剪，其他零面积矩形表示空裁剪；正面积极小矩形
   保留相交像素。单独记录 clip 是否启用，不把空像素范围当成取消裁剪。
5. GUI 的 Save/Restore、结构边界与绘制变换隔离保持不变。RenderWidget 会消除
   目标自身位置，因此回归渲染包含小数位置子控件的父容器，保留真实布局偏移。

## 参考与取舍

Flutter 3.38.1 的 [Clip 定义](https://github.com/flutter/flutter/blob/3.38.1/engine/src/flutter/lib/ui/painting.dart#L1191)
区分图元和裁剪 AA，普通 AA clip 还可能带来合成边缘问题；本次保留硬裁剪。
Qt 6.8.3 的 [矩形 clip 实现](https://github.com/qt/qtdeclarative/blob/v6.8.3/src/quick/scenegraph/coreapi/qsgbatchrenderer.cpp#L2392)
也存在就近取整路径，不能据此推断通用路径与贴边圆环的覆盖行为。
原始 [NanoVG glnvg__fill](https://github.com/memononen/nanovg/blob/ce3bf745eb2d2dbc14a50bf2446783f691ac4353/src/nanovg_gl.h#L1011)
不在 fringe 阶段消费模板，因此它的原路径 bounds 清理不能直接用于 fork 的去重算法。
修正位于 [NanoVG v0.0.14 renderFill](https://github.com/golang-gui/nanovgo/blob/v0.0.14/gl_backend.go#L858)。
这些是源码行为与实现取舍，不是其他框架对 GOUI 裁剪的保证。

## 验证

- 纯换算与 Software 离屏：整数／小数／负坐标、1／1.25／1.5／2x、小尺寸、
  空裁剪与替换、预乘颜色和窗口局部 clip 不随绘制 transform 改变。
- GUI 录制替身确认最终逻辑交集不取整，逻辑分离但包络重合的子项不参与绘制。
- `tests/window/widgets/progress_regression -case=clip`：48／64 DIP、4 DIP 环宽，
  整数／四分之一／半物理像素偏移，128／255 透明度、四倍率、亮暗及 1／.25／.75
  进度。按独立圆面积及圆端帽预期检查边缘；另对空裁剪恢复、旋转、1 物理像素边框、
  相邻半透明填充、嵌套结构和 ScrollView 视口作明确像素断言。
- 反向验证：恢复旧 OpenGL 裁剪后，48 DIP 半像素圆环边缘 alpha=0，预期约 .5；
  保留新裁剪、恢复 NanoVG v0.0.13 仍失败。NanoVG 清理范围回归在旧实现上失败，
  新实现及其全仓测试通过。
- Linux amd64 / X11 / Xfwm4 合成开启：Software 与 OpenGL（Mesa llvmpipe）
  实际后端通过全部24组矩阵；最终标签依赖重建后的 OpenGL 1x／2x、Software 1.25x
  原生窗口再次通过，桌面半像素顶边像素约为红128／64（前景alpha255／128，黑底），
  符合约半覆盖。原有 `-case=all` 的完整环、帧驱动和隐藏／恢复回归两后端通过。
  Software 1.25x 的既有 `pixel_snap` 窗口完成点击、亮暗切换和 HeaderBar 40→41 DIP
  更新，普通边框、小数高度按钮及圆角呈现正常。
- Windows 10 x64：Direct2D／Software 1x 原生窗口各通过24组矩阵并检查桌面呈现，
  Direct2D WARP 离屏全包原生测试通过。
- Direct2D 旧测试把正面积 .2×.2 DIP 当作取整后空裁剪；依据新契约改为真实零宽
  输入，保持空栈项可替换的原断言，正面积极小裁剪由新增回归独立覆盖。
- 最终 `go test ./... -count=1 -json`：6245项测试／子测试通过，0失败、0条件跳过；
  155个无测试文件的包另计。既有X11包内测试使用私有窗口／selection。
  `env -u DISPLAY go test -race ./platform/graphics/internal/pixelsnap ./platform/graphics/software ./gui -count=1`
  通过；`go mod verify` 通过。Windows amd64、macOS arm64／amd64 的绘制包、GUI、
  两个受影响窗口入口交叉编译通过，不能代替原生窗口验收。
- 未通过／待验收：Windows `GOUI_PLAT_PAINTER=opengl` 的
  `progress-final.exe -case=clip -hold` 在创建窗口时返回
  `create opengl context err: can not choose useful pixel format`；该环境未运行GL场景。
  macOS 无可用登录桌面，未执行窗口验证。
- 移动目标只通过纯像素换算包交叉编译，未作原生运行验收。
  `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build ./platform/graphics/internal/pixelsnap ./platform/graphics/software ./platform/graphics/opengl`
  失败于 purego 的 `undefined: cgo.Dlopen`；相同命令切换 `GOOS=ios` 失败于
  `_PUREGO_REQUIRES_CGO_ON_IOS`。本轮环境缺少这些后端要求的目标cgo构建条件，
  不以纯Go交叉构建覆盖移动原生图形验证。

## 旋转圆弧的细分与接合覆盖率

ApiBox 历史窗口的48 DIP、4 DIP环宽在1x仍有明显台阶感。固定旋转角度并与独立
圆面积预期比较，OpenGL误差大于Software，原因进一步定位为两处：

- `bevelJoin` 把路径接合点的U坐标固定为对称描边中心的 .5。填充AA使用内1.5、
  外.5物理像素的不对称边带，真实路径点应映射为 .75，约半覆盖；错误的满覆盖
  会在细分变密、触发内接合分支时产生端帽局部突起。
- NanoVG 的同一 tessTol 在圆弧细分中作为线性误差，在贝塞尔判定中作为平方
  阈值使用，物理像素误差尺度不一致。统一为1/16物理像素的线性容差，贝塞尔
  比较时平方。两个控制点必须投影到弦段内，最大垂距不能超过容差；容差胶囊的
  凸性保证整段曲线在界内。重合端点仍检查控制点，保留闭合曲线及共线折返；
  达到有限细分上限时保留终点，上限和浮点精度仍限制极端输入的误差保证。
- 更密的轮廓会超过原128边的尖端面积覆盖预算，使已有尖端回退到 fringe 近似。
  预算改为192边，Go 与 shader 共用同一常数；不改变复杂／相交路径的选择规则。
  192个边向量、11个画刷向量及其他参数低于 GLES 3.0 最低224个 fragment uniform
  vectors，也低于GL3.3最低256个的保证。保留144边多轮廓尖端回归，旧预算明确失败。

NanoVG 分别提交接合覆盖率和物理像素细分修复，发布 v0.0.15；GOUI 固定该标签。
资源归属、结构裁剪和应用的尺寸声明保持原契约，不为 ProgressBar 添加后端补偿。

参考 Skia chrome/m139 的 [物理像素细分误差约束](https://github.com/google/skia/blob/chrome/m139/src/gpu/tessellate/Tessellation.h#L28)：
它使用1/4物理像素限制折线近似误差。本次采用明确物理误差的思路；GOUI 的
1/16由圆弧与轨道合成的质量预算和细分成本共同决定，不承诺不同后端像素相同。
uniform 最低保证见 [GLES3 glGet](https://registry.khronos.org/OpenGL-Refpages/es3.0/html/glGet.xhtml)
及 [GL glGet](https://registry.khronos.org/OpenGL-Refpages/gl4/html/glGet.xhtml)。

### 后续圆弧验证与成本

- NanoVG `go test ./...`、`go test -race ./...` 通过。独立三次曲线求值检查到
  输出折线的物理距离，覆盖四倍率、两种尺寸、非均匀变换、反曲线、闭合、共线
  折返及退化点；旧细分判定和旧接合坐标均在新增断言上失败。
- GOUI 正式v0.0.15依赖下 `go test ./... -count=1 -json`：6245项测试／子测试
  通过，0失败、0条件跳过，155个无测试文件包另计；既有X11包内测试使用私有
  窗口／selection。`go mod verify` 及
  `env -u DISPLAY go test -race ./platform/graphics/software ./gui ./widgets -count=1`
  通过。Windows amd64、macOS arm64／amd64绘制包、GUI、widgets和两个受影响窗口
  入口交叉编译通过；不能代替原生窗口验收。
- `progress_regression -case=arc` 检查48／64 DIP、0／.25／.5物理像素相位、
  四倍率、七角度、亮暗轨道和128／255前景alpha，共56阶段、672圆弧。
  每个边缘像素对照64×64独立圆面积积分及预乘 sRGB RGBA8 的 source-over
  预期；保留12%覆盖差及2级量化预算（32.6），精确clip和边框仍使用原1级容差。
  此预算包含折线近似、线性AA和面积AA的差异，不能解释为精确面积渲染承诺。
- Linux amd64 / X11 / Xfwm4 合成开启 / Mesa llvmpipe OpenGL：56阶段全通过，
  最大通道误差30.547；Software 同矩阵最大10.797。正式tag重建后，两后端的
  原生2x窗口再次通过全部56阶段。原有 `-case=all` 的完整环、
  帧驱动及显隐恢复，两后端通过；OpenGL `-case=clip` 和既有 `corner_radius -auto`
  的四倍率、尖端、渐变、擦除和阴影断言通过，未扩大原容差。
- ApiBox 历史窗口实际spinner位于半像素高度，48 DIP、4 DIP环宽；亮暗原生动画
  与三态尺寸检查通过。固定73度的同场景对照，最大RGB误差22.76→10.99，边缘
  平均误差4.23→1.65，误差超过16的像素22→0。这是单角度实测，完整角度覆盖
  以上述独立窗口矩阵为准。
- 同环境旧→新→旧CPU基准，每组5次、每次300ms，预热并复用路径cache：
  48像素单段三次弧194.0→468.1 ns，64像素191.3→473.7 ns，反曲线424.9→1013 ns；
  点数9→17、9→17、19→35，全部0 B/op、0 allocs/op。整窗离屏对照每组5×20次，
  中位数旧7.09 ms→新7.43 ms→旧6.87 ms，增幅约5%～8%；范围包含FBO创建、绘制
  和CPU读回，不能外推为屏幕帧耗时或其他GPU性能。
- Windows 10 x64 / Direct2D / 1x 的 `progress.exe -case=arc` 首阶段未满足32.6预算：
  48 DIP、phase=0、alpha=255、angle=0，像素(54,24)的RGBA为(9,11,14,16)，
  独立面积预期约(28.20,34.35,43.82,50.40)。用旧NanoVG v0.0.14重建后得到
  完全相同像素；Direct2D不使用NanoVG，这是新检查揭示的既有独立质量差异。
  保留失败断言，不放宽预算或跳过；后续单独分析Direct2D边缘覆盖率。
- Windows OpenGL 原生像素格式仍不可用，macOS 无可用登录桌面，移动端未原生验收。
  交叉编译和纯逻辑测试不能补足这些窗口验证缺口。
