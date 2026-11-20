# 小数位置绘制裁剪

状态：小数裁剪已完成；旋转圆弧本轮完成本地修复与验证，尚未发布新标签。

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

## 旋转圆弧视觉质量复核

v0.0.15降低了部分误差，但不能据此认定history动画的视觉质量已经解决。前轮
32.6通道误差预算允许近似边带差异，七个固定角度也没有覆盖相邻帧的稳定性。
本轮先在本地验证，不发布新NanoVG标签，不以单个角度或单项测试通过作为验收。

实施范围：NanoVG内部填充覆盖率、原有ProgressBar窗口回归和本文件；不改变
Widget API、ProgressBar几何、应用布局或其他绘制后端。验证以实际history动画、
独立圆面积预期及Software对照共同判断，并回归细边框、小圆角、孔洞、透明合成、
裁剪和擦除。普通逻辑测试、性能对照及原生窗口观察分别记录。

### 修正与边界

- 通用填充的线性fringe是距离近似，不等于方形像素内的轮廓覆盖面积。仅加密
  贝塞尔不能改变该近似；直接扩大边带的试验还破坏了细边框和小圆角，未采用。
- 凹轮廓、带孔轮廓和已有尖端面积路径改为对边界像素积分。每条原始边生成
  保守四边形，覆盖可能相交的像素中心；相交边带都计算同一个轮廓面积，通过
  模板高位使每个像素只合成一次，其他位填充原始轮廓内部，最后完整清理模板。
  保留原凸轮廓快速路径，不将全部图元改成同一种渲染器。
- 只有所有非零区域具有同一方向的单位绕数时才能取积分绝对值。自交、接触、
  叠加绕数及正负区域混合仍使用既有模板路径，避免孔内反向岛在像素内抵消。
- 按物理像素行筛选相关边，RGBA32F表代替固定192边uniform。Context拥有一个
  复用纹理及CPU缓冲，逐帧覆盖数据，容量按需增长；排队命令记录只读范围，
  取消帧丢弃范围，Context销毁释放纹理和两个绘制程序。不同Context不共享可变数据。
  容量受原生纹理上限和float32精确索引范围约束，超出时保留既有填充方式。
- 面积绘制程序与常规绘制程序分开，前者不使用discard；面积计算限定在边界
  四边形内。OpenGL 3.3／ES3使用相同算法，WebGL保留原填充方式。
- 同时独立修正模板读取掩码污染写入掩码缓存的问题；这不是spinner根因的替代证据。

Skia chrome/m139的
[凸路径覆盖实现](https://github.com/google/skia/blob/chrome/m139/src/gpu/ganesh/ops/AAConvexPathRenderer.cpp#L335)
明确说明距离近似对极细区域的限制；本轮据此保留绘制路径的适用边界，未照搬其
凸路径公式。面积积分仍基于离散轮廓，不承诺解析圆或所有后端逐像素相同。

### 细分复核

线性物理单位和退化曲线修正仍成立，但前轮没有充分区分几何误差与覆盖率误差。
本轮分别对照1/4、1/8、1/16、1/32及更细候选，并以独立圆面积、Software及实际
history角度序列判断。单独加密至1/256仍留下覆盖率误差，不能作为修复。

默认改为1/32，与面积修正独立提交。48 DIP、4 DIP环宽、半像素高度的history在
七角度对照中，v0.0.15最大RGB误差9.99～11.17；只改面积、保留1/16时为5.17～6.74；
1/32时为3.00～4.28，边缘平均误差0.66～0.70，接近Software的0.66～0.71。
这些是具体场景数据，不作为所有路径或所有设备的误差承诺。

同环境5次×300ms、预热并复用cache，48像素单段三次弧的CPU细分中位数：
v0.0.14为190.8 ns／9点，v0.0.15为467.4 ns／17点，1/32为929.0 ns／33点；
1/16复核为467.7 ns。全部0 B/op、0 allocs/op，细分成本确实增加。

### 本地验收与交付状态

- `go test ./...`及临时modfile引用本地NanoVG的全仓测试均6246项通过，0失败、
  0条件跳过；154个无测试文件包另计。临时工作区不传给构建命令的独立模块测试。
  NanoVG全仓及race、Software／GUI／widgets race通过。
- `progress_regression -case=coverage`：43角度、四倍率、亮暗、三种像素相位、
  两种尺寸和透明度，共344阶段。独立多边形裁切参考的RGBA误差不超过2级，
  相邻.25度样本的差值误差不超过4级；实测最大分别0.527、1.006。旧v0.0.15
  在该直边断言失败，证明此回归不依赖新增细分常数。
- OpenGL 1x／原生2x覆盖率专项通过；圆弧相对独立圆面积的最大通道误差9.265。
  Software的`-case=arc-motion`344阶段最大10.797。Software严格直边试验未满足
  2级预算，26.6输入及扫描转换与浮点积分不同；不计为面积专项通过，不扩大本轮
  修复范围，也不以放宽专项容差解决。原完整环、动画显隐恢复、裁剪、圆角、
  尖端、渐变、擦除、阴影和销毁回归保持通过。
- Linux amd64／X11／Xfwm4合成开启／Mesa llvmpipe OpenGL：同一history实际
  动画，首帧后1秒预热、8秒被动采样，旧→新→旧：1x为57.67→57.75→57.78 FPS，
  2x为35.75→35.67→34.42 FPS。本场景未观察到帧率回退，不外推到其他GPU或路径规模。
  固定角度截图序列用于视觉对照，不冒充帧率录像。
- 本地Mesa GLES3的两种程序编译、链接，以及RGBA32F行表的整覆盖／半覆盖／
  空覆盖像素检查通过；不能代替Android／iOS完整窗口及生命周期验收。
- Windows amd64、macOS arm64／amd64受影响绘制包及窗口入口交叉编译通过；
  本轮未执行对应原生窗口或移动端验收。Direct2D前述独立质量差异仍待处理。

NanoVG本地修复与容差调整分别提交；本轮不打新标签、不更新GOUI正式依赖。
默认依赖仍为v0.0.15，复核实际应用需使用包含本地NanoVG的临时工作区。
