# Tooltip

Tooltip 是 Widget 的简短说明，由 GUI 根据鼠标停留或键盘导航自动显示。
本文是当前实现契约，取代 DesignUI / Roadmap 中尚未确定的 Tooltip 草案。

## 公开接口

`Widget.Tooltip() string` / `SetTooltip(string)` 保存普通文字；空字符串移除提示。
WidgetBase 只保存文字，未挂载设置不创建计时器或原生对象。Name 表示名称，Tooltip
表示补充说明，二者互不推导。调用方负责本地化，语言改变后重新设置普通字符串。

声明式共享 `.Tooltip(text)` 只协调文字。沿用 Name 的初始值快照：显式空值清除；
省略修饰器恢复挂载时的控件默认值。没有可见性绑定、Tooltip Widget 或内容构建回调。

## 交互

- 从实际几何命中点向祖先寻找最近的非空提示，保留裁剪及前后遮挡规则。
  禁用控件仍可说明不可用原因，不因此接收点击或普通 Hover 信号。
- 初次停留 500 ms；已显示提示后 1 秒内浏览下一个目标使用 100 ms。
  等待时超过 4 DIP 的移动重新计时；显示后同一目标内的移动不使浮层追随指针。
- Tab 导航获得 FocusVisible 时延迟显示；鼠标点击获得焦点不触发提示。
  Escape 或其他按键、按下鼠标、滚轮、拖放、离开目标、窗口失焦／隐藏关闭提示。
  这些输入继续原有分发，Tooltip 不消费或重放输入；Escape 后原目标不会立即重开。
  原生浮层显示可重新报告静止鼠标位置；没有坐标变化的 MouseMove 不切换键盘提示。
- 显示期间保留提示，直到交互或生命周期使其失效，不设置强制阅读超时。
- 目标文字改变时旧会话失效并重新等待；设置为空立即关闭。隐藏、卸载、销毁、
  移动到其他宿主时取消等待。布局后重新核对命中、可见区域及焦点。
- 每个 Window 至多一份提示状态；普通窗口和它的当前模态 Popover 使用同一所有者。
  模态目标切换先关闭旧提示，不让后台窗口内容的提示盖住模态操作。

## 呈现与资源

复用非模态 Popover，不新增输入穿透、焦点或原生 Tooltip 接口。提示优先位于目标
可见区域下方；工作区不足时使用已有 Bottom 定位的翻转／平移。提示不获取焦点，
点击提示自身仅关闭提示，不承诺把该次点击递交给它下面的控件。

正文使用 Label 的原生排版，支持换行与自动折行，最大宽度 360 DIP，尺寸受工作区
约束，不增加滚动条。Basic / Modern 提供 `tooltip`（表面）和 `tooltip-text`
（文字）规则；留白为布局值。GUI 显式将目标的样式环境用于提示，不改变普通
Popover 默认使用应用样式的规则。主题改变复用既有样式失效及宿主布局通知。

窗口私有状态拥有目标、触发来源、代次、一个 GUI Timer 和一个当前 Popover。
Timer 使用 StartOnce，不创建独立线程；到期检查目标、代次、Root、窗口状态和
输入位置。切换与取消先失效旧状态，再销毁旧原生对象；回调返回后再次检查代次。
关闭／销毁窗口停止计时并清除引用，Tooltip 不延长控件或旧 Root 的生命周期。

原生 Tooltip 表面创建时自身的 Hidden／失焦通知，不作为锚点窗口的生命周期事件。
取消必须核对事件所属宿主，不能把同一 owner 下任意 Popup 的初始化状态混为一谈。

显示失败属于辅助呈现的尽力而为失败：释放部分资源，本次停留不循环重试，也不阻断
原始输入或弹出错误窗口。原生 Popover 的错误返回契约保持不变。

## 语义

WidgetInfo.Tooltip 保存说明文字，提示未显示时也可读；不替代 Name。
可见提示子树使用 tooltip 角色、真实文字及边界，没有 Click / Focus 动作。
使用 Window.Popovers 枚举显示的提示，不增加自动化执行接口。

## 参考与取舍

- GTK4 4.24 的 [tooltip-text](https://docs.gtk.org/gtk4/property.Widget.tooltip-text.html)
  与 Qt 6 的 [QToolTip](https://doc.qt.io/qt-6/qtooltip.html)：采用控件上的简单文字入口。
  GTK 的 [query-tooltip](https://docs.gtk.org/gtk4/signal.Widget.query-tooltip.html)
  提供按区域查询；本轮不引入动态内容协议。
- Qt 6 [qtooltip.cpp](https://code.qt.io/cgit/qt/qtbase.git/tree/src/widgets/kernel/qtooltip.cpp?h=6.10)
  使用延时、事件过滤和关闭管理提示。借鉴延迟与取消的职责划分，不照搬全局对象。
- Windows [TTTOOLINFOW](https://learn.microsoft.com/en-us/windows/win32/api/commctrl/ns-commctrl-tttoolinfow)
  的 TTF_TRANSPARENT 是可选消息转发行为；它不证明普通 Tooltip 必须具有通用输入穿透。
  GOUI 本轮通过定位及关闭策略处理交互，不扩展 platform 能力。

以上为参考文档／源码，时间、尺寸、触发及清理规则以本文为准。

## 验证

包内使用可推进的计时器测试等待、抖动、目标切换、禁用状态、取消后的过期回调、
焦点、重入、销毁、样式及声明式默认值恢复。窗口程序通过正常 DispatchEvent
验证点击、滚轮、菜单、Tab、屏幕边缘和布局变化；真实桌面补充鼠标及截图检查。
Linux、macOS、Windows 记录后端与缩放；交叉编译不代替原生运行。
声明式手工入口为 `tests/window/ui/tooltip`，使用正常 UI 协调及真实桌面输入。
用带构造默认提示的 Button 区分显式空串与省略修饰器；重建后检查提示文字、
控件身份、未受控输入及选区，另行观察提示呈现。延时更新用于核对已显示提示的失效，
隐藏／卸载和重新挂载用于核对生命周期。切换主题时复用已有控件，不重建另一套提示机制。
GUI 窗口程序的 `-manual` 仅提供真实桌面操作，不运行模拟断言或输出 PASS。Windows 原生
Popup 显示可能重新报告真实鼠标位置；真实与模拟位置不一致时，后来的系统输入应生效，
不能为了测试丢弃真实移动。自动输入与真实桌面验收分开记录，不把失败改记为通过。
