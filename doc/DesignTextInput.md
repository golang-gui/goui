# TextInput 遮蔽与视图输入策略

## 边界

TextInput 增加 Obscured / ObscureChar；TextInput 与 TextView 共用 InputFilter。
共享 TextModel 保持无输入策略的文档；策略归属视图，程序赋值和历史回放不经过策略。
UI 只协调值，不独立过滤或维护编辑状态机。首版不增加多行遮蔽或平台安全输入声明。

参考 Flutter 官方 inputFormatters 的顺序组合与只处理用户编辑的契约，以及 GTK4
set_invisible_char / Flutter obscuringCharacter 的自定义替代字符能力：
- https://api.flutter.dev/flutter/material/TextField/inputFormatters.html
- https://api.flutter.dev/flutter/material/TextField/obscuringCharacter.html
- https://docs.gtk.org/gtk4/method.Text.set_invisible_char.html
Flutter TextField 构造实现明确限制 obscured 字段为单行；GOUI 也先集中支持单行，
不将复用编辑内核作为强制给 TextView 增加遮蔽的理由。
- https://api.flutter.dev/flutter/material/TextField/TextField.html

## 输入策略

`gui/textedit.InputFilter` 为函数策略，不是信号。GUI 对外重导出该类型，两个编辑控件
均有 InputFilter / SetInputFilter；nil 不过滤。替换策略只影响后续用户编辑，不修改
正文、历史、选区或 IME。函数不比较身份，UI 更新可重复设置，无清理或通知副作用。

Input 是借用的编辑上下文，暴露替换范围、插入文本及只读的当前文本／候选文本查询；
不暴露 Model 指针，不强制拼接全文。调用期间模型必须保持不变，不能保留上下文。
策略返回处理后的插入文本及是否接受；范围由编辑操作决定。策略应是纯计算，不修改
控件、模型或策略；GUI 仍防护外部代码销毁、卸载、修改正文或选区后提交过期编辑。

Chain 按声明顺序将结果交给下一项，任一拒绝即拒绝整次编辑；nil 子项跳过。
内置 LimitLength、AllowRunes、DigitsOnly、ASCIIUsername、Password：
- LimitLength <= 0 不限；按 Unicode 扩展字素限制，超限裁剪本次插入的完整字素前缀。
  保留选区外文字，完全无法接收新增内容时不将选区删空。已有超限内容允许缩短或
  不增加字素数的编辑。删除不受限，组合字符连接必须按完整候选文本判断。
- AllowRunes：检查新增字符，存在不允许字符即拒绝；不悄悄删除字符。nil 谓词不限。
- DigitsOnly：ASCII 0-9。ASCIIUsername(n)：ASCII 字母、数字、下划线、点、连字符及上限。
- Password(n)：允许 Unicode（含空格、Emoji），拒绝 Unicode 控制字符及长度超限。
  不要求字符组合／最小长度；不自动开启遮蔽。完整业务校验留给提交操作。

用户输入、IME 最终提交、粘贴、替换、删除、剪切都在模型修改前经过统一入口。
预编辑不限制；拒绝不修改正文／历史／选区，组合结束仍按原生事实退出预编辑。
剪切先确认编辑可接受，再写剪贴板。撤销／重做准确恢复历史，不重新过滤。
TextInput 保持单行规范化，TextView 保留 LF；过滤结果在 GUI 边界重新规范化。
无策略不创建上下文或候选全文；全文或全局长度策略的扫描成本由显式选择该策略承担。

## 遮蔽

Obscured 默认 false；ObscureChar 默认 U+2022，可设为 * 等单个可显示 rune；0 恢复默认。
控制、空白、组合标记、格式控制字符和非法 rune 不作为替代字符，非法设置 panic。

模型和 Text / ConnectText / BindText 使用真实文本。原生排版默认接收每字素一个
替代字符；可选反馈期间仅保留显露的单个字素，映射不保存完整原文。将原生度量中的 UTF-8 位置映射回投影原文
坐标，复用 Cluster 的命中／几何／选择／导航，不假装圆点字节偏移就是模型偏移。
IME 投影先组合，再生成遮蔽排版；绘制、预编辑下划线、滚动及原生候选定位共用此几何。
按词导航／双击将遮蔽内容当作整体；普通左右和删除仍按映射后的 Cluster 边界。

遮蔽禁止复制／剪切，保留粘贴、选择、删除和历史。Snapshot 正文／可见文字／预编辑
正文不输出原文，TextEditingInfo.Obscured 标记隐藏；长度和选区仍是原文 UTF-8 单位。
不把替代文字放进按原文范围解释的 VisibleText。退出遮蔽恢复普通文本语义。
切换模式或替代字符只清理视图排版／几何和更新菜单，不改模型／历史／逻辑选区。
不引入内置眼睛按钮，不承诺 Go 堆内存清零或系统级安全输入。

### 末字素短暂显露

TextInput.ObscureDelay / SetObscureDelay(time.Duration) 控制可选反馈，默认 0，非正值
均为立即遮蔽。与 Obscured、ObscureChar 独立，不自动启用遮蔽。推荐应用按需要设置
600ms 左右；窗口测试使用 2 秒，以便真实输入与声明式重建的观察。

- 仅成功提交的单个完整字素触发；过滤前提交与过滤后插入都须是单字素。一次 IME
  最终提交一个字素可触发，预编辑始终遮蔽。组合符连接到既有字符时显露整个结果字素。
- 复制／剪切及 Snapshot 的保密契约不变。粘贴、程序赋值、历史、多字素提交不显露。
  不从 TextChanged 猜测输入来源；只有 IMContext 的提交入口启动反馈。
- 新提交先隐藏前一次反馈；最多显示一个字素。选区／光标改变、控件／窗口失焦、隐藏、
  禁用、只读、卸载、正文变更及遮蔽配置改变均取消反馈。重新聚焦或重新启用不恢复。
- 相同 setter 为 no-op，声明式重建不重启计时；相同正文赋值不清除反馈。到期仅改变
  显示，不修改模型、选区、历史或输入版本，也不让等待中的粘贴失效。
- GUI 按需创建并复用一个单次 Timer，卸载停止并释放引用；默认关闭时不分配计时器。
  Timer 失败确定性退化为立即遮蔽，已排队的旧通知由现有 Timer 取消／重启契约防护。
- 混合显示记录显露区域的显示字节范围，度量转换时恢复源字素边界；原生字体回退
  分裂的同一显露字素合并几何。到期重新排版，命中、光标、滚动均使用当前显示度量。

参考：GTK4 的 gtk-entry-password-hint-timeout 默认 0，并建议启用时可用 600ms；
Qt 6 提供 SH_LineEdit_PasswordMaskDelay。GOUI 采用控件实例上的显式时长，不新增
平台设置查询或主题行为策略。
- https://docs.gtk.org/gtk4/property.Settings.gtk-entry-password-hint-timeout.html
- https://doc.qt.io/qt-6/qstyle.html#StyleHint-enum

## 声明式接入

TextInput.Obscured(bool) / ObscureChar(rune) / ObscureDelay(time.Duration) / InputFilter(filter)。
TextView.InputFilter(filter)。省略恢复 false／默认替代字符／0／nil；不向状态源回写转换后的程序赋值，不比较函数身份。
协调时先配置显示／策略，再绑定文本和选区。输入经过 GUI 的同一条用户编辑路径。

```go
password := gui.NewTextInput()
password.SetObscured(true)
password.SetObscureChar('*') // Omit to use U+2022.
password.SetInputFilter(textedit.Password(128))

// Declaration: secret retains the real value; only presentation is obscured.
ui.TextInput().BindText(&secret).
    Obscured(true).
    InputFilter(textedit.Password(128))
```

## 验证

- 策略：字素、组合边界、选区替换、部分接收、控制字符、顺序组合、借用失效和长文本。
- GUI：真实分发删除／粘贴／快捷键、IME 提交与预编辑、拒绝后的历史／选区不变、
  程序赋值和回放旁路、共享模型独立策略，以及回调重入／销毁防护。
- 遮蔽：原生布局不含原文、映射与选择、复制剪切禁用、菜单／快照、切换及资源释放。
- UI：省略恢复、重建不重置历史／IME、绑定写回及不比较函数。
- 本机独立 X11 会话，以 Software / OpenGL 的 1x / 2x 验证像素、输入、滚动、预编辑
  及剪贴板。窗口用例记录操作和独立断言，不使用 DevServer 验收。
- `tests/window/ui/textinput_obscure` 默认手动验收：正常 Modern 主题、自然测量高度，
  保留焦点、选区与光标。`-auto` 才启用统一边框、透明光标及等尺寸视口的像素断言，
  并检查视口足以容纳原生行高；`-auto -hold` 可保留测试外观供原生输入像素检查。

实际执行与未验收项见 [Plan](Plan.md#textinput-遮蔽与共享输入策略2026-11-03)。
