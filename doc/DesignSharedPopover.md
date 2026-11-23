# Popover 与 SharedPopover

本文记录 2026-11-17 确认的收敛设计，并于 2026-11-23 扩展分层呈现宿主，替代独立共享 Root 结构。
实现与验证进度见 [Plan](Plan.md)。

## 两个概念和唯一实际宿主

Popover 是完整的 GUI 弹层，拥有 Root、控件树、布局、输入、焦点、帧、原生 Popup 和
Painter。SharedPopover 是轻量客户端，仅保存配置与内容引用，并协调使用最近呈现宿主的缓存。
不增加 ReusedPopover 或另一种实际 Root。

Window 与普通 Popover 的私有共享子槽各缓存一个普通 `*popover`；槽本身不实现 Root、
不嵌入 rootBase。独立创建的普通 Popover 不占窗口共享槽，也可提供自己的共享子槽。
每层的 Root、Popup 和 Painter 在正常反复显示和同层客户端切换中保持同一身份。

`Widget.Root()` 返回实际普通 Popover。它是借用对象；能进行类型断言不等于取得缓存
宿主的所有权。SharedPopover 句柄不实现 Root，Destroy 永远不销毁共享宿主。

## 公开接口

保留 `NewPopover(anchor, options)` 和 `NewSharedPopover(anchor)`；两者增加 SetAnchor。
构造、SetAnchor 不创建原生资源，锚点可为 nil 或尚未挂载，Show 校验并返回失败。
其他内容、样式、位置、Placement、Modal、AutoDismiss、输入和信号接口保持原有用途。

SetAnchor 设置不同锚点时先更新请求配置，再结束旧显示，不自动重新显示。Closed 回调
可以显式 Show，此时使用新配置；旧清理不得覆盖回调产生的新显示。相同锚点不操作。
不重置 Position、Placement、样式和内容。解除旧锚点的卸载连接。

## Popover 的资源归属

首次有效 Show 解析当前锚点的 Surface。归属是当前原生资源的归属，不永久绑定 Go 对象。

- 同 Surface 的 Hide/Show 或更换锚点复用实际 Root、Popup、Painter 和挂载内容。
- 锚点卸载只结束显示和输入，不释放有效原生资源；同窗重新挂载后可继续使用。
- 更换到另一 Surface 后，下次 Show 先卸载依赖旧 Painter 的内容，释放旧原生资源、
  撤销旧 Surface 登记，再在目标 Surface 创建并重新挂载。Go 对象继续使用。
- Surface 保存所有已归属 Popover，包含隐藏实例，独立于可见枚举；销毁时统一释放。
  普通 Popover 对象与内容按既有卸载契约保留，之后可换到新 Surface。
- 显式 Destroy 永久终止对象，释放资源、连接和引用。Surface 的专用缓存最终 Destroy。
- 原生 Popup 意外终止释放本次资源；后续 Show 可重建。创建失败清理部分资源。
- 显示前的查询/布局失败保持隐藏，保留仍有效的资源；不把普通错误当作重建理由。

资源释放遵守线程亲和和既有 native/surface 代次，旧 Surface 的回调不能影响迁移后资源。
锚点不能直接或间接形成循环；解析失败而非无限追溯。

## 共享槽与仲裁

共享槽记录缓存 Popover、当前客户端、保留内容的客户端、请求代次和过渡状态。首次
SharedPopover.Show 才创建。每次显示都完整设置当前内容、锚点和全部配置，含默认值。

B 替换 A：校验目标和模态准入 → Hide A → 完成关闭通知 → 检查代次 → 卸载旧内容 →
应用 B 配置及内容 → 调用普通 Popover.Show。若 A.Closed 打开 C，B 取消并保留 C。
返回可用 errors.Is 判断的 ErrPopoverSuperseded；真实原生错误保留原因。

Hide 只结束自身显示，保留至多一份挂载树；同客户端重开不重复挂载。换客户端才卸载。
Destroy、锚点卸载或替换已缓存的内容只移除自身缓存和连接，不销毁原生资源。旧客户端
的设置、输入、焦点、Hide/Destroy 不能影响后来客户端。内容卸载不隐式 Destroy Widget。

SharedPopover 迁移到另一呈现宿主时改用目标缓存，源缓存留下；不同于普通 Popover 跨窗
释放重建自己的一套资源。嵌套锚点选择父 Popover 的子槽，不替换父层；同层客户端仍
不能依赖即将卸载的内容。Tooltip 不抢占菜单和列表，后两者打开时取消待显示提示。
级联按每宿主一级的方式复用，不把一条菜单链塞进窗口同一个槽。

所有外部回调后复核生命周期、请求和原生代次。共享槽内部关闭观察者先撤销旧身份，
再转发客户端关闭；已完成清理后允许 Closed 重入。绘制或不安全过渡中的同步抢占
返回错误。保留普通 Popover 既有合法的焦点/悬停关闭重入，不清空全部信号掩盖问题。

## 通用显示与平台职责

普通与缓存 Popover 使用同一个 Show，不再有 show(false)/show(true) 两条路径。
隐藏阶段 Measure → 提交尺寸 → ClientSize 查询实际 DIP/像素 → Arrange → 按锚点与
工作区定位。移动改变 backing scale 时有限地重新确认尺寸，未收敛不得显示。

显示前旧 backing 必须清除或失效；清除不执行 Widget.Paint 或用户帧信号。显示提交
可见性和输入归属后，经合法原生 Repaint 处理首帧。允许内容绘制慢，但位置/大小已稳定，
等待期间只允许干净背景或透明内容。Repaint 不承诺合成器扫描显示完成。

保留已验证的通用后端修复：GLX 同 Context 的固定 pbuffer 承担帧外资源操作；
DirectComposition 解除旧 Visual 内容并 Commit/WaitForCommitCompletion，成功 Present
后重新挂接；Software 转发其原生上传呈现失效。后端不感知 SharedPopover 或抢占。
不以等待、坐标补偿、反复重绘或重建原生窗口代替资源与呈现正确性。

共享缓存首次创建沿用透明能力协商，仅创建阶段的 Unsupported/Unavailable 能力失败
允许改为不透明配置；布局、定位和普通 Show 错误不得触发能力降级。

## 隐藏期间的失效与原生通知

隐藏实例保留原生资源，不代表仍有可用锚点。共享客户端归还后锚点可以为空；普通
Popover 的锚点也可以卸载、重新挂载或改到另一 Surface。样式、内容和位置变更在
隐藏期间只更新配置并标记失效，不测量内容、不查询工作区、不提交原生尺寸或位置。
下一次 Show 先解析当前锚点与资源归属，再按最新样式测量、查询实际尺寸和定位。

隐藏期间收到原生 SizeEvent 仍记录实际 DIP／像素尺寸并标记待布局，但不能按旧定位
快照移动窗口或调度隐藏帧。Show 准备阶段的原生尺寸与定位仍遵循上面的通用显示流程。
可见实例继续响应样式变化并报告真实工作区查询错误；不能通过忽略所有布局错误，
或销毁缓存资源来处理隐藏锚点暂不可用的正常状态。

参考 Qt 6.8.3 桌面 Widgets 的
[QWidgetPrivate::update](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/kernel/qwidget.cpp#L11292)：
不可见 Widget 不提交 backing store 更新，renderToTexture 分支仅保留失效标记。
这里采用隐藏失效与原生提交分开的原则；Popover 的锚点、工作区查询和重新显示顺序
仍由 GOUI 自己的 Surface／Show 契约决定。

## 输入、快照与消费者

内容通过实际 Popover 的原有 EventDispatcher/Controller 处理；SharedPopover 只在自身
持有当前使用权时转发。隐藏已呈现的宿主不接收输入或非 nil 焦点，也不持续调度帧。
保留无锚点、从未显示的惰性 Root 的无桌面测试能力。

Surface.Popovers 继续返回绑定每次显示的 PopoverTarget，结束或重开使旧目标失效；
其内部直接指向普通 Popover。它是枚举/输入视图，不是额外宿主。
菜单、DropDown、Tooltip 只配置并使用共享句柄，不参与原生借用、回收、首帧准备。
IME、模态、拖放统一识别普通 Popover；ui 不另建调度或宿主体系。

## 验收与迁移

包内覆盖同/跨 Surface 换锚点、卸载重挂、隐藏资源最终清理、共享迁窗、配置/输入隔离、
Closed/焦点/悬停重入、帧中销毁、失败、原生失效和几何/像素。旧的“实际 Root 不实现
Popover”断言改为“实际 Root 是缓存普通 Popover，SharedPopover 句柄不实现 Root”，
继续保留过期输入目标和所有权隔离断言。

原生验收：Linux X11 Software/OpenGL 各 1x/1.5x/2x；Windows Direct2D/Software
各 1x/2x，OpenGL 单独记录可用性；macOS Software/OpenGL 各 1x/2x。包含大小菜单、
下拉/右键/Tooltip、普通弹层共存、屏幕边缘、慢绘制、锚点切换和跨 Surface 迁移。
复用按资源身份和创建/销毁计数判断，不以进程 RSS 必须回落替代资源断言。
执行完整 go test ./...，分别记录通过、失败、跳过和环境缺口，不弱化旧断言。

NewTransientPopover 仍已移除。外部旧依赖需将该构造及字段一并迁移为 SharedPopover；
不修改外部仓库或模块缓存来伪造兼容。

## 参考与证据

- Qt 6.8.3 [QComboBoxPrivate::viewContainer](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qcombobox.cpp#L1058)
  缓存既有 Popup 容器，Hide/Show 沿用宿主；不声称 Qt 采用 GOUI 的 Surface 单槽。
- GTK 4.16.13 [Popover realize/unmap](https://github.com/GNOME/gtk/blob/4.16.13/gtk/gtkpopover.c#L1010)
  区分原生资源和可见映射；不引入 GTK 事件循环。
- GTK 4.16.13 [GLX 帧外目标](https://github.com/GNOME/gtk/blob/4.16.13/gdk/x11/gdkglcontext-glx.c#L228)、
  Qt 6.8.3 [QOffscreenSurface](https://github.com/qt/qtbase/blob/v6.8.3/src/gui/kernel/qoffscreensurface.cpp#L20)
  是帧外目标和实际格式兼容的参考。
- Microsoft [WaitForCommitCompletion](https://learn.microsoft.com/en-us/windows/win32/api/dcomp/nf-dcomp-idcompositiondevice-waitforcommitcompletion)
  等待合成引擎处理先前 Commit，不保证扫描显示完成。


## 分层呈现宿主的共享槽

窗口与普通 Popover 各自延迟持有一个共享子槽。锚点属于哪一个实际 Root，
就使用该 Root 的槽；原生所有者仍由 `anchorSurface` 解析到窗口。菜单的父子层
因此可以同时显示，同层客户端继续抢占和复用。`SharedPopover` 不实现 Root，
不承担原生资源所有权。

父宿主 Hide 或替换内容先递归隐藏子槽，保留各层的普通 Popover、Painter 和原生资源；
客户端 Destroy 只解除客户端及内容。实际宿主 Destroy 递归释放缓存链。
跨原生所有者时旧缓存释放，不把原窗口的资源迁给新窗口。缓存数量随曾使用的最大深度
增长，与访问过的菜单项数量无关。短暂绘制目标失效不等于逻辑宿主销毁。
