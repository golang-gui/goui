# 小数位置绘制裁剪

状态：修复已完成，适用后端的验证结果与环境缺口见下方。

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
  被写入的模板像素。GOUI 固定依赖该标签，不含本地 replace 或控件补偿。

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
