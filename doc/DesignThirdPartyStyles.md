# 第三方控件样式注册

## 边界

第三方控件必须提供 Basic 外观，可以额外提供 Modern 外观。注册入口分别由
`theme/basic`、`theme/modern` 提供；`style` 继续只表达规则、属性与解析，不管理主题。
其他第三方主题由相关项目自行约定。GOUI 不导入第三方控件，不维护主题标识、适配器
匹配、优先级图或降级链。控件使用 `gui.ResolveWidgetStyle` 和既有 `StyleChanged`。

## API 与启动时序

```go
basic.Register(name string, rules func(base style.StyleSheet) []style.Rule)
modern.Register(name string, rules func(options modern.Options, base style.StyleSheet) []style.Rule)
```

扩展在包的 `init` 中分别注册；应用正常导入控件并选择 Modern 即可。名称使用模块或包路径，
同一名称可以在两个主题各注册一次。空白名称、nil 函数、同主题重名、首次构造后再注册均 panic，
用于暴露初始化错误。`Register` 只保存函数，不执行函数或构造完整主题。

首次调用各主题的 `Rules` / `Sheet`（含间接调用）冻结该主题的注册集合。
Modern 构造还会调用 Basic，因而也会冻结 Basic。无运行时注册、卸载或测试 reset API。
GUI 默认 Basic 使用 `sync.OnceValue(basic.Sheet)`，延迟到首次使用，避免 gui 包初始化早于
扩展的 init 而漏掉注册。不要在包级变量初始化时构造主题；正常入口在应用启动/build 中。

每个主题内部按注册名称排序。锁只保护登记和快照，不在调用扩展函数时持锁。
构造可并发执行，扩展函数因此必须可并发调用、确定且无副作用；只返回所属控件命名空间的
普通样式值，不访问 GUI、原生对象、应用或系统设置，不调用完整的 Rules / Sheet。
不得依赖其他扩展的注册顺序。返回规则由本次构造独立拥有，不返回共享可变切片或颜色对象。

Basic 回调的 base 是完整的内置 Basic（含 widgets），不含任何扩展规则。
Modern 回调的 base 是内置 Modern，不含 Basic 扩展或其他 Modern 扩展。
base 是只读快照，可解析官方控件的前景、边框、字体、选区等；不暴露内部 palette。
Modern 的 options 已归一化：无效字号变为 14 pt，主题色被复制为不透明颜色，其他字段为当前值。
回调不能保留外部可变颜色对象或通过全局设置推断 Dark。

## 组合与覆盖

`basic.Rules` 返回：内置 Basic → Basic 扩展。

`modern.Rules` 返回：完整 Basic → 内置 Modern → Modern 扩展。

只有 Basic 实现的控件，在 Modern 中自然保留其 Basic 规则；这只是明确的规则拼接，
没有运行时查找或适配器选择。每次 Rules 返回独立切片；Sheet 仅保存生成后的规则，
不保存构造回调。旧 sheet 不随新选项改变，声明式层现有的深比较与替换机制继续有效。

应用把自己的规则追加在主题 Rules 后，再调用 `style.Sheet`。同 selector 后规则覆盖先规则；
part/state 的既有解析优先级不变，不承诺最后一条低优先级 selector 可以覆盖所有状态。
应用将 stylesheet 设置回 nil 时使用包含扩展的完整 Basic。

```go
func init() {
    basic.Register("example.org/control", func(base style.StyleSheet) []style.Rule {
        return []style.Rule{{Sel: style.Sel{Name: "example.control"}, Style: base.Resolve(style.Sel{Name: "label"})}}
    })
    modern.Register("example.org/control", func(o modern.Options, base style.StyleSheet) []style.Rule {
        return []style.Rule{{Sel: style.Sel{Name: "example.control"}, Style: base.Resolve(style.Sel{Name: "label"})}}
    })
}
```

## 验证与依据

包内测试验证非法/重复/过晚登记、排序、只读基础快照、归一化选项、切片及旧 sheet 隔离。
独立进程 fixture 验证 gui 比扩展更早导入时的默认缓存，以及仅 Basic / 双适配在 Modern 下的结果。
相关主题、GUI、UI、widgets 测试检查既有主题覆盖与声明式行为。

接口采用 Go 包初始化时显式登记的常见形式；[database/sql.Register](https://pkg.go.dev/database/sql#Register)
对重名与 nil 使用 panic 是错误语义的参考，并不复用其驱动体系。
[Avalonia ControlTheme](https://docs.avaloniaui.net/docs/styling/control-themes)（2026-04-20 文档）
表明控件行为与主题呈现可分离；这里只采用职责划分，不引入资源键查找、继承或类型匹配。
