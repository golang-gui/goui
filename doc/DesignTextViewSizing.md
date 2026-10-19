# TextView 内容测量与单行排版一致性

## 契约

TextView 继续使用唯一的 Measure / Arrange 布局路径。Measure 在本次有效宽度下
排版，返回包含 Padding、服从父约束及自身 Min/Max 的内容高度和首行基线；
不再固定申请六行。空文档占一行，末尾 LF 的空行计入高度，软换行只影响显示。
预编辑投影参与显示测量，Placeholder 不参与内容范围。

ScrollView 在现有 Measure 中使用子项的测量结果申请视口尺寸，再接受自身
Min/Max 和父约束。不增加自然尺寸查询、测量模式、属性开关或高度变化信号。
宽松约束允许随内容增高；MaxSize 限制增长；紧约束或父布局的 MainWeight
分配决定固定/填满视口。默认 MainWeight 仍为 1，紧凑字段显式设置为 0。

Scrollable 内容的 Measure 只决定期望视口，ContentSize 继续提供完整滚动范围，
LayoutVisible 使用最终视口和偏移。受限的测量结果不能覆盖完整范围。
普通内容仍按既有滚动方向释放滚动轴以取得完整范围。滚动条占位沿用现有求解。

单行视觉一致指同主题、字体、Padding 和一行分配高度下，TextInput 与 TextView
具有相同文字基线、Placeholder、光标、选区与输入外观。文档正文顶部对齐；
TextView 被父层强制拉高时不因第一行/第二行的切换改变整份文档的对齐策略。
TextInput 继续在其单行分配中垂直居中。

## 排版与失效

两控件共用稳定字体行度量及段落首行基线规则。段落的文字位移同时用于绘制、
光标/选区、命中、IME 和视口锚点；原生指标较高时完整容纳，避免裁掉回退字体。
原生行指标与编辑 Geometry 分开缓存，编辑索引仍只在命中、导航、选择时构建。
Basic / Modern 的 text-view 与 text-input 使用一致默认输入外观，保留独立名称。

测量使用当前文档/预编辑、宽度、字体和 WrapMode；变化沿现有 RequestLayout
失效到父层。颜色和闪烁只绘制。有限高度下确定已达到上限后允许停止；无界时
计算完整高度。滚动范围继续保留未测量段落的估计，并在可见布局中修正。
测量不移动视口、选区或焦点；保留段落缓存上限和控件资源隔离。

## 参考与取舍

- GTK4 在线文档标注 4.24.1 的
  [自然高度传递](https://docs.gtk.org/gtk4/method.ScrolledWindow.set_propagate_natural_height.html)
  与 [高度上限](https://docs.gtk.org/gtk4/method.ScrolledWindow.set_max_content_height.html)
  区分完整内容和受限视口。GOUI 采用该职责区分，通过既有 Measure / MaxSize 表达。
- Flutter 官方 [maxLines](https://api.flutter.dev/flutter/material/TextField/maxLines.html)
  文档区分字段显示高度和可输入内容。GOUI 使用 TextView + ScrollView 组合。
- VS Code 1.105.0 的
  [InputBox](https://github.com/microsoft/vscode/blob/1.105.0/src/vs/base/browser/ui/inputbox/inputBox.ts#L514)
  区分内容高度和最大显示高度；GOUI 使用原生排版尺寸，不引入 DOM 镜像。

上述是文档/源码依据，不是跨框架运行对照。验证结果另记 PlanTextViewSizing.md。
