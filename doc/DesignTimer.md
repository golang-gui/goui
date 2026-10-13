# GUI Timer

## 1. 定位与接口

普通定时通知属于 GUI 执行环境，不要求底层对应原生计时器。所有用户回调在 GUI
主线程执行；不做帧动画时钟、任务优先级、跨线程 Timer 操作或 UI 生命周期绑定。
不增加 platform Timer API。Timer 明确归属于创建它的 Application，不绑定 Window
或 Widget。Application 提供执行环境并在退出时停止调度，调用方负责不再需要时 Stop。

```go
type Application interface {
    // ...
    NewTimer() *Timer
}

func (t *Timer) ConnectTimeout(fn func()) signal.Handle
func (t *Timer) Start(interval time.Duration) error
func (t *Timer) StartOnce(delay time.Duration) error
func (t *Timer) Stop()
func (t *Timer) Active() bool
```

- `app.NewTimer()` 创建 inactive Timer，永久绑定 app，不注册任务或申请等待资源。
  不再提供包级 `gui.NewTimer`。零值 Timer 不能启动，Stop 无操作，Active 为 false。
- `Start` 周期执行，`StartOnce` 单次执行；允许在所属 Application.Run 之前调用。
  时间从 Start 调用时开始计算，重复启动替换旧任务，不再查找全局 App 或重新绑定。
- 时长必须大于零。Timer 未绑定 Application、调度已关闭或时长无效时返回错误；
  校验失败不改变原任务。并发 Quit 会使整个调度失效，不能继续保留活动任务。
- `Stop` 幂等，取消尚未开始的通知，但不打断已经开始的信号及其剩余监听者，
  不断开信号连接；之后可以重新启动。不增加 Destroy。
- 单次任务发出信号前即 inactive；周期任务在回调期间仍 active。
- 所有 Timer 方法遵循 GUI 线程亲和；其他 goroutine 通过 Application.Post 操作。
  Timer 使用后不能复制。只有 Application.Quit 允许直接从其他 goroutine 调用。
- 活动任务被调度器保留，不能依赖丢弃 Timer 引用来取消任务。控件应在适当的
  unmount／销毁路径 Stop；窗口或控件不额外登记 Timer 所有权。

使用示例（Application 创建后，在 GUI 线程执行）：

```go
timer := app.NewTimer()
timer.ConnectTimeout(func() {
    // Update widget state on the GUI thread.
})
if err := timer.Start(500 * time.Millisecond); err != nil {
    return err
}
// When no longer needed:
timer.Stop()
```

GUI 层以 `Application.NewTimer` 为唯一创建入口，不提供包级 TimeoutFunc／TickFunc。
UI 层的运行时便捷方法见 §6；它们绑定传入的 App，不查找全局 Application。

## 2. 共享调度与线程边界

采用 **到期最小堆 + 一个 channel 型 time.Timer + 一个等待 goroutine**。
不用 AfterFunc，不为每个逻辑 Timer 创建 channel 或 goroutine，不用 reflect.Select。
标准库负责等待与唤醒；GOUI 维护逻辑任务的到期次序与 GUI 分发规则。

`gui/timer.go` 提供公共对象，Timer 的启停／查询方法只向所属 Application
转交请求。`application` 的私有 `startTimer`、`stopTimer`、`timerActive` 方法统一
接入 `gui/timer_scheduler.go`。Application 创建时初始化调度状态，Run 时才启动
唯一等待 goroutine。

Timer 持有所属 application、信号和私有 `*timerRun` 任务句柄。公共 Timer 方法不读写
句柄或任务内容；只有 Application 绑定任务与 Timer，并提供 `t.timeout.Emit` 作为
内部通知函数。任务结束后句柄可以保留至下一次 Start／Stop，但不再保留通知函数。

调度器只认识自己的任务和普通函数，不接收 `*Timer`，不访问 Timer.run、信号或
Application。其内部入口为 `schedule(previous, interval, once, callback)`、
`cancel(run)`、`active(run)`；Application 保存 schedule 返回的句柄，Stop 时取消
任务并清空句柄。启动校验及旧任务替换仍在同一次加锁中完成。

运行实例仅记录 callback、单调时钟 deadline、interval、单次标志和堆索引，不持有
Timer 或 scheduler 指针。callback 为 nil 表示任务取消或完成，同时释放通知目标的
引用；任务是否有效不再依赖 `timer.run == run`。周期续订复用同一实例，不引入额外
map、active 字段或代次。

内部通知函数可能通过闭包或方法值引用 Timer 的信号，但调度器无需知道其类型和
内部结构。这不是新增公共 callback API：用户仍通过 ConnectTimeout 接收信号。
调度器继续作为 gui 的私有实现，不额外提取成通用 executor 或公共调度框架。

| 状态 | 访问规则 |
| --- | --- |
| Timer 的 run 句柄 | Application 在 GUI 线程管理，调度器不读写 |
| Timer 的信号、用户回调 | GUI 线程 |
| 任务 callback、堆、堆索引、closed／started／pending | 调度器互斥锁 |
| 底层 time.Timer 的创建、C、Stop／Reset | 唯一等待 goroutine |

堆中任务的 deadline 只在移出堆后、重新入堆前更新。堆顶查询 O(1)，插入、取消、
续订 O(log N)。相同 deadline 的任务不保证顺序。

另有容量为 1 的 changed channel。队列状态先在锁内同步更新，再非阻塞发送通知；
通知仅让等待者重新检查状态，可以合并，不承载异步启停命令。

等待过程：

1. 在锁内读取最近 deadline，将共享 time.Timer 设置为等待这一期限。
2. 队列为空或已有未开始的 GUI 分发通知时，停止底层计时，仅等待 changed。
3. 到期后重新检查当前队列，排除停止／重启造成的旧唤醒。
4. 标记 pending，解锁后通过已有 Post 投递一次分发任务。

底层 Timer 的 Stop／Reset 使用 Go 1.23 起的同步 channel 语义（项目 go 1.24），
不支持通过 `GODEBUG=asynctimerchan=1` 恢复旧语义。没有定时轮询，空闲时不周期唤醒。
不持有调度锁调用 Post 或用户代码。

## 3. GUI 分发、取消和周期

一次 GUI 分发在锁内最多取出 64 个到期任务，清除 pending，解锁并唤醒等待者，
然后逐个调用任务通知函数，由 Application 提供的函数发出信号。超出部分留给
下一次分发。始终最多有一条尚未开始的定时
分发通知，GUI 忙碌时后台不会反复累积唤醒。

Stop 经 Application 请求调度器清空任务 callback、依据堆索引移除任务，然后由
Application 清空 Timer.run 句柄。不在堆中的索引为 -1。
已经取出的批内任务，在调用前检查 callback 是否仍存在及调度器是否关闭：

- 同批次 A 可以取消或重启 B。
- 已投递但尚未执行的通知可以取消。
- 旧运行实例不会触发重新启动后的任务。
- 不需要额外 active 字段、取消标志或递增代次。

单次任务在锁内取出通知函数并清空 callback，解锁后调用，因此其 Active 在通知
开始前即为 false。周期任务在通知期间保留 callback，因此仍为 active。取消／关闭
不会打断已经取出的、获准执行的本次通知及其信号监听者。

周期任务在信号返回前不重新入堆，避免同一运行实例在原生嵌套循环中重入。
返回后检查任务 callback，仍存在且调度未关闭才续订；回调内停止或替换任务会
清空旧任务 callback，阻止旧任务续订。固定节拍以原 deadline 为基准，
跳过已错过的时刻，不补发历史通知。例如周期 100ms：

- 100ms 的回调在 130ms 返回，下一次是 200ms。
- 回调在 350ms 返回，下一次是 400ms。

全部排期使用保留单调时钟信息的 time.Time，不转换为墙上时间。不存在实时精度保证；
GUI 繁忙时通知会延后，慢回调仍会阻塞 GUI。

不设置全局“正在分发”禁令：一个回调进入原生嵌套事件循环时，其他尚未取出
的 Timer 仍能分发。本批中后续任务仍需等当前回调返回。显式重新 Start 是新的
运行实例；若随后进入原生嵌套循环，新实例可以按现有 Post 语义执行。

## 4. 应用接入与退出

- Application 内部持有调度服务，公共接口仅新增 NewTimer，不暴露调度器或运行实例。
- Run 前已到期的任务，开始处理事件循环后执行一次，不回放错过的周期。
- Quit 先关闭调度，再请求平台事件循环退出；最后一个窗口关闭也走 Application.Quit。
- Run 返回时幂等关闭调度并等待唯一等待 goroutine 退出。Quit 本身不等待回调或 worker。
- 关闭在锁内清空堆中任务的 callback 并释放注册引用，不从后台修改 Timer.run 或信号。
  已取出的批内任务在继续分发／续订时检查 closed 并释放 callback。Active 在关闭后
  一律为 false。已开始的信号可返回，未开始的批内任务和旧投递全部失效。
- 普通 Post 的退出规则不变；定时投递额外受调度 closed 状态保护。
- Settings 的 2 秒轮询复用同一调度服务，替代单独的 Ticker + goroutine + Post。
  使用传入 Application 的 NewTimer 和正常的 ConnectTimeout／Start／Stop，
  不接触调度器或私有启动路径。原生查询仍只发生在 GUI 分发中，Run 返回时 Stop，
  兜底 Settings 不创建任务。

Linux `libc.Poll` 从 RawSyscall 改为 Syscall：阻塞等待必须向 Go runtime 报告线程
进入系统调用，使其他 goroutine 能调度并通过已有 self-pipe 唤醒事件循环。
不改变 X11 先处理原生事件、后处理 Post 的顺序，不修改非阻塞管道 Read／Write。

## 5. 测试与验证

确定性测试使用私有 now／Post 注入，不创建等待 goroutine：覆盖单次、周期、
取消已投递任务、重启、同批次失效、回调内启停、固定节拍、延迟跳期、批量上限、
唤醒合并、嵌套分发、堆索引移除和 Settings 查询线程边界。
测试通过 Application.NewTimer 及公开启停接口驱动，不再注入 scheduler 到 Timer。
额外覆盖零值拒绝启动、固定 Application 归属、通过 Application 接口创建、
单次／周期行为、首次延迟、返回对象取消和重启、错误路径不留下活动任务。

独立调度器测试只提交普通函数，不创建 Application、Timer 或信号：验证单次任务在
通知前失效、非法替换保留旧任务、批内取消、回调内替换、周期续订和关闭时释放通知
函数。调度器的有效性判断不需要外部对象配合。

真实等待测试覆盖共享等待、空队列重新启动、并发 Quit、Run 前启动和退出清理。
原生集成测试在独立子进程的原始主线程创建 Application，运行单次／周期 Timer；
Linux 使用 GOMAXPROCS=1、关闭异步抢占且不创建窗口，验证无输入时的唤醒。
父进程设独立超时，避免错误的阻塞系统调用连 Go 自己的测试超时都饿死。

```sh
go test ./...
go test -race ./gui -run 'TestTimer|TestSettings|TestApplicationQuits' -count=1
go test ./gui -run '^$' -bench '^BenchmarkTimerScheduler$' -benchmem
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -exec=true ./...
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go test -exec=true ./...
```

基准分别测量 1／100／1000 个任务的启停、周期续订及批量分发。它们不包含绘制，
也不代表相比其他计时方案的性能提升。信号自身的快照分配不属于计时器续订分配。
Windows／macOS 的真机测试、原生移动／缩放和模态嵌套循环体验需在对应平台验收；
交叉编译不替代原生执行。

本轮 Linux 验证：新增测试及 race 检查通过，Windows amd64／macOS arm64 全包
交叉编译通过。临时恢复旧 RawSyscall 时，原生 Timer 子进程在单 P 场景中超时；
恢复 Syscall 后通过，确认该绑定修正不是仅凭推测。

`go test ./...` 仍有原有 `platform/test.TestApplicationIdentity` 失败：不同 appId
创建返回的 typed nil 被包入接口后不等于 nil。该问题不在计时器修改范围；排除
该测试后其余测试通过。

## 6. UI 层运行时接入

UI 参考 SwiftUI 的声明式使用体验，但定时器本身不作为 View 声明。`OnClick(f)`
在 Build 中只是登记回调，重建不会执行 f；事件回调中创建 Timer 不会因重建而重复
启动。不增加 OnTick／OnTimeout 修饰器，不改 View／BuildContext／协调器。

```go
// ui.App
TimeoutFunc(delay time.Duration, fn func()) *Timer
TickFunc(interval time.Duration, fn func()) *Timer

// ui.Timer
func (t *Timer) Start(interval time.Duration)
func (t *Timer) StartOnce(delay time.Duration)
func (t *Timer) Stop()
func (t *Timer) Active() bool
```

- 便捷方法完成创建、绑定回调和首次启动，始终返回非 nil 的 `*ui.Timer`，不返回
  error。不公开 NewTimer、ConnectTimeout 或信号句柄。
- Start 切换到周期运行，StartOnce 切换到单次运行，均替换旧任务并重新计算期限，
  保留最初的回调。Stop 幂等，不断开回调，可以重新启动；不打断已开始的回调。
- 所有方法可从任意 goroutine 调用，通过所属 App.Sync 同步转交 GUI 线程。
  回调始终在 GUI 线程执行。其他节拍、跳期、Active 语义沿用 gui.Timer。
- 非正时长、nil 工厂回调在调用方立即 panic，不改变已有任务。应用退出后创建返回
  失效句柄，合法启停无操作，Active 为 false；nil／零值 Timer 同样安全失效。
  GUI Timer 已由所属 Application 正确创建且参数已验证，剩余的启动失败来自调度
  关闭，UI 层按退出时无操作处理。
- 不根据 Build 更新最初的闭包。需要读取新数据时捕获持久业务状态；确实需要更换
  回调时停止旧 Timer，再创建新的 Timer。使用后的 Timer 不应复制。
- 调用方在不再需要时 Stop，应用退出最终兜底。View 移除不自动停止业务 Timer。
  可在已有 Window.OnDestroy 中停止窗口相关任务，不另加生命周期注册表。
- 通知本身不请求重建。更新 ui.State 使用 Set／Update，修改普通 Go 状态后调用
  App.RequestUpdate。不要在 Build 中无条件调用工厂，也不要把返回句柄保存在每次
  Build 都重新创建的临时 View 值中。

示例：持续存在的业务对象保存状态与 Timer，反复点击时复用计时器。

```go
type toastModel struct {
    shown ui.State[bool]
    dismiss *ui.Timer
}

func (m *toastModel) show(app ui.App) {
    m.shown.Set(true)
    if m.dismiss == nil {
        m.dismiss = app.TimeoutFunc(3*time.Second, func() {
            m.shown.Set(false)
        })
    } else {
        m.dismiss.StartOnce(3*time.Second)
    }
}

// m 在 Build 外创建并持续存在；这里只声明事件处理函数。
ui.Button("Show toast").OnClick(func() { m.show(app) })
// 业务结束或所在窗口销毁时，调用 m.dismiss.Stop()。
```

### 实现与验证

`ui.Timer` 只保存所属 UI app 与 `*gui.Timer`。通过绑定的 gui.Application.NewTimer
创建，回调连接一次，不保存第二份回调、运行标志或期限，不访问 GUI 调度器。
初次 UI Build 失败也调用 GUI Quit，清理在原生 Run 之前启动的任务。

App.Sync 的跨线程调用等待操作完成或 App 运行结束；排队操作在执行前检查停止状态。
App 的 finished 通道仅在 run 结束时关闭，不能在 Quit 请求时提前关闭，否则正在
执行的同步操作可能仍在写入调用方结果。该机制由 App 共用，Timer 不自行实现退出
等待。同步等待只阻塞调用者，不为每个 Timer 创建 goroutine。

UI 单元测试覆盖参数校验、失效句柄、同步调用等待与退出、回调内 Quit。原生子进程
测试在原始主线程运行 ui.Run，从其他 goroutine 创建／查询／启停 Timer，验证单次、
周期、重复启动复用回调、回调内启停、模式切换、显式状态更新、重建不重启以及初次
Build 失败的真实 GUI 任务清理。不为 UI 测试向 GUI 暴露调度器或额外工厂接口。

```sh
go test ./...
go test -race ./ui ./gui -run 'TestTimer|TestAppSync|TestSettings|TestApplicationQuits' -count=1
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -exec=true ./...
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go test -exec=true ./...
```
