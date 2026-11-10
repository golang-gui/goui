# 列表增量更新与表格编辑

## 目标

在表格末行输入时追加空行，保留原单元格的 Widget、焦点、选区、撤销和正在进行的
预编辑。结构通知不再无条件卸载全部可见行；不增加输入框焦点补偿或另一套 UI 协调。

## 模型与通知

保留 ListModel 的 ItemsCount / ConnectItems。可选 ListModelChanges 增加
ConnectItemsChanged(func(ListChange))，ListChange 包含 Position、Removed、Added、
Reset。通知在修改完成后同步发送；范围使用修改前的位置，之后的索引平移。
Reset 表示没有可靠的增量描述，原有模型仍使用完整重载。零删除/增加只刷新展示。

SliceListModel 的旧、新订阅共享同一个底层信号，每次修改只发布一次。Append、
Insert、Remove、Set 提供准确范围；Set 是单项替换，Modify/SetItems 不猜测泛型值
的身份，使用 Reset。模型的私有代次检查将过期的重入通知转换为 Reset。
存储并发安全不等于 GUI 可以跨线程使用；界面模型仍在 GUI 线程发布变更。

嵌入 SliceListModel 而自定义 ConnectItems 的模型，也会通过 Go 方法提升具备精确
通知能力；若包装的是订阅/通知语义，应一并包装 ConnectItemsChanged。只公开旧
ListModel 接口的适配器仍使用旧路径。测试计数模型同时统计实际使用的通知入口，
保留对连接次数/泄漏的断言，不将新入口绕过计数当成“没有重复安装”。

ListView 只订阅一种通知，优先使用精确能力。校验范围和前后数量；通知过期或无法
应用时完整重载。保留范围外的已挂载 Widget，平移索引和测量缓存；只解绑被删除或
替换的行。后续 Bind 更新数据/索引，但不通过 Unbind 清空保留行的声明子树。
滚动仍按视口回收，不固定无限数量的编辑器。请求中的 Reveal 跟随保留项平移，
目标被删除则取消。回调重入、销毁或重装模型后停止旧批次，收敛到当前模型。

## TableView 与 UI

TableView 已拥有稳定模型内 RowID。比较更新前后的 ID 序列，保留相同前缀和后缀，
对中间区间发送一次替换；末尾追加只报告新增范围。ID 不变的数据变化 Refresh。
业务 TableModel 无须实现精确通知；即使用 Modify 一次写入文字并追加空行，内部
适配器仍根据 RowID 得到可靠范围，不将业务模型的 Reset 直接转发为行重载。
刷新保留行时使用最新 TableRow 和单元格声明；原列 ID 和原 Widget 类型继续通过
既有 UpdateChildren 协调。无需给每个编辑器附加新的 ID，也不添加 Root 特例。
若应用显式声明编辑器 ID，同一业务记录下应保持该声明身份；不要把会随前置插入
变化的行索引当作长期标识。

移除、替换、滚出视口或换列时继续正常 Unbind 和焦点移交。任意排序/复杂重排可能
替换整个中间区间，本轮不承诺移动中的编辑器始终保留，也不引入完整 keyed diff。
应用仍需在 Build 外保存 Model、保持 RowID 稳定，编辑值保存在业务模型/状态中。

## 参考与取舍

- [Gio ListModel items-changed](https://docs.gtk.org/gio/signal.ListModel.items-changed.html)
  （Gio 2.x，自 2.44）：公开位置、删除数量和增加数量；后续索引随数量变化。
- [Qt 6 QAbstractItemModel](https://doc.qt.io/qt-6/qabstractitemmodel.html#beginInsertRows)
  区分插入/删除与模型重置，维护对应索引。这里参考其结构更新区别，不复制 Qt
  的索引对象、编辑器框架或成对 begin/end 协议。

以上为官方 API 契约；GOUI 的可选能力、代次防护与有限行回收是本项目的实现选择。

## 验证与实施

包内集中于 gui/list_view_test.go、widgets/table_view_test.go、widgets/ui/table_view_test.go
及 ui/list_view_test.go：增删替换、前后索引、缓存、旧模型兼容、连接清理、重入、
焦点与对象保留、选区/撤销、快照和视口回收。窗口例程用正常输入分发连续输入并
追加空行，另提供真实键鼠和中文 IME 的手工步骤；不使用 DevServer。

阶段：先实现/验收通用列表，再接入表格与声明式窗口，最后运行隔离全仓、定向 race、
跨平台编译及独立 X11。原生中文 IME、远端与移动端结果按实际执行情况分别记录。

## 验证记录（2026-11-10）

- 基线316db52，Linux amd64 / Go1.25.8，已跟踪源码叠加本轮改动的仓库外副本。
  独立 Xvnc/Xfwm4 执行 `CGO_ENABLED=0 GOWORK=off go test -json -count=1 ./...`：
  **4632通过、11既有跳过、0失败**；跳过集合与上一轮一致。未删除/纳入用户的旧
  未跟踪例程和测试；旧测试的窗口及图片写入副作用只发生在副本/独立会话。
- `go test -race ./gui ./ui ./widgets ./widgets/ui -run
  'Test(ListView|SliceListModel|Table|TextValueBindings)' -count=1` 及受影响包和新窗口
  例程的 go vet 通过。普通通知兼容测试显式隐藏新能力，继续断言完整重载；增量
  模型另验证准确范围、缓存、Reveal、保留身份和唯一释放，没有削弱原兼容覆盖。
- 新窗口例程的 Software/OpenGL（Mesa llvmpipe）×应用1x/2x均通过自动断言，
  NewImage 的实际资源类型确认后端。连续输入只追加一次空行；重建、选区替换、
  撤销/重做、前置插入/删除、删除编辑行及新的末行输入全部通过，正常保留期间
  lost=0。不使用 DevServer。2x 为应用覆盖，不代替系统混合 DPI 或硬件 GPU。
- OpenGL 1x 另以独立会话原生鼠标/键盘输入 abc、选取b、F4重建、输入X、
  Control+Z与Control+Shift+Z；F6依次断言/报告abc→aXc→abc→aXc，始终2行、
  原输入框、lost=0，截图复核焦点框与两条记录。正常关闭退出码0，独立会话已清理。
- 将 widgets/table_view.go 单独恢复为基线源码 overlay，运行同一个窗口例程，
  首字符追加时 panic“原编辑器身份或焦点丢失”，负对照符合预期；新源码通过。
  普通 ListView 的首个新增回归也在基线代码下复现“追加行解绑了原有可见行”。
- `CGO_ENABLED=0` Windows amd64/macOS arm64 的 gui/...、ui、widgets/...、Demo及
  新例程交叉编译通过。本轮未执行两端原生窗口、系统中文输入法、移动表格或设备
  输入；预编辑保留只计入 GUI IME 替身的状态断言，不描述为系统候选窗验收。
- 额外尝试裸 GOOS=android/ios 的 CGO_ENABLED=0 包构建时，分别遇到 purego 的
  runtime/cgo.Dlopen/Dlsym/Dlclose 和 _PUREGO_REQUIRES_CGO_ON_IOS 限制。它们是
  已在 [移动计划](PlanMobileAPI.md)记录的非正式构建路径限制；改用正式 cmd/goui，
  以 gui/activation 的 `-X main.viewMode=ui` 构建 Android ARM64 APK 与 iOS ARM64
  Simulator unsigned app 均通过。此为 GUI/UI 消费者构建，不计作移动表格运行。
- 开发中：旧菜单属性通知必须保持 Reset；初版仅刷新数据使既有菜单自然宽度测试
  失败，改由正常 Modify 发布 Reset 后通过。一次测试重入触发器自身缺少一次性
  防护产生递归，修正辅助代码后保留重入断言；不归为生产已确认崩溃。窗口脚本曾
  将 stderr 的 PASS误作 stdout漏报，最终例程统一输出stdout；首轮原生键入沿用
  宿主fcitx设置而独立会话没有该服务，配置本地XIM并点击输入框后完成验收。这些
  失败/环境结果没有算作通过，也没有改动原生输入实现来兜底测试环境。

日志、负对照、截图、构建产物与隔离源码保存在 `/tmp/goui-table-edit-f7q943_x/`。
本轮未作性能提速结论；一般排序的中间区间替换不承诺保留进行中的编辑。
