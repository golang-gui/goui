# 安装 GOUI 应用开发 Skill

本文件是给执行安装的 AI Coding 工具阅读的说明。用户不需要手动查找技能目录、
复制文件或为 GOUI 配置 Agent 适配器，只需发送：

> 请按照 https://github.com/golang-gui/goui/blob/main/skills/INSTALL.md 的说明，安装 GOUI 应用开发 Skill。

英文提示：

> Install the GOUI application development skill following https://github.com/golang-gui/goui/blob/main/skills/INSTALL.md.

## 安装目标

将官方仓库 `golang-gui/goui` 的 `skills/goui-app/` **完整目录**安装为名为
`goui-app` 的技能。当前内容如下；以后安装时以所选版本的实际目录为准：

```text
goui-app/
├── SKILL.md
└── references/
    ├── application.md
    ├── build-and-debug.md
    └── custom-widget-view.md
```

入口遵循 [Agent Skills 格式](https://agentskills.io/specification)。不同工具的
支持情况、目录和加载方式由工具自身决定；本文件不承诺所有 AI 工具均支持 Skill。
`INSTALL.md` 是安装说明，不是需要注册的第二个技能。

## 给安装 AI 的操作说明

### 1. 确认当前工具的安装机制

- 识别当前 Coding 工具及其版本，优先使用已有的原生技能安装工具。
  没有安装工具时，查阅该工具的官方说明或本地配置，确认实际支持的技能目录。
  不根据另一款工具的目录猜测，也不要在多个候选目录中各放一份。
- 用户已指定项目级或全局安装时遵从其选择；否则采用当前工具文档明确的默认范围，
  并告知用户。只有范围或路径无法确定、权限不足时才询问缺失信息。
- 如果当前工具不支持这种技能格式，如实报告限制并停止安装。可以在当前任务中
  经用户同意直接阅读技能，但不能将此描述为已安装；不擅自改写 AGENTS.md、
  系统提示、MCP 配置或创建产品专属适配器。

### 2. 获取官方技能目录

- 来源仅为 [GOUI 官方仓库](https://github.com/golang-gui/goui)。遵从用户指定的
  tag、分支或 commit；未指定时使用 `main`。读取一次后确定来源 revision，
  尽可能以该 commit 获取全部文件，避免把不同版本的入口和参考混在一起。
- 原生安装器支持仓库子目录时，提供仓库和 `skills/goui-app` 路径。
  否则在仓库外临时目录下载该子目录，或做临时稀疏检出，再安装其完整内容。
  只有经用户确认的本地 GOUI 源码副本才可替代远端来源。
- 先检查下载内容及其相对引用。不要只下载 SKILL.md；不要把整个仓库当作技能，
  不执行仓库脚本，不安装 GOUI 的 Go 依赖，不修改应用 go.mod 或启用开发服务。
- 文件不存在、网络失败或来源无法确认时，报告具体原因。尚未发布到所选 revision
  不等于工具不支持 Skill；不要换用来历不明的镜像或声称安装完成。

### 3. 安装并保留已有内容

- 目标应是该工具识别的技能目录下的 `goui-app`，里面直接包含 SKILL.md，
  不要多套一层 `skills/goui-app`。目录名与 frontmatter 中的 `name` 保持一致。
- 先检查目标。内容一致时报告已安装，不重复写入。存在不同内容时，先告知来源
  和差异；未得到更新授权，不覆盖用户已有技能或本地修改。
- 使用工具的安装机制，或只复制已检查的技能文件；不需要管理员权限、额外插件
  或 GOUI 专属安装脚本。失败时保留原技能，不留下被工具识别的半份安装。

### 4. 验证与反馈

- 检查 SKILL.md 的 YAML frontmatter，确认 `name: goui-app`、非空 description，
  所有本地参考存在且可读，内容与所选来源一致；有原生校验工具时运行它。
- 工具支持枚举已安装技能时，确认 `goui-app` 被发现；否则报告已完成文件安装，
  但尚未验证运行时发现。不要用文件存在代替“本会话已加载”。
- 按工具实际机制说明是否需要刷新、重启或新会话。可给出通用使用提示：
  “请使用 goui-app Skill，帮我用 GOUI 编写一个桌面应用。”
  只有工具支持 `$技能名` 语法时才推荐 `$goui-app`。
- 最终报告安装范围、实际目录、来源 revision（拿不到 commit 时说明这一限制）、
  检查结果和下一步。安装完成不意味着 GOUI 应用已构建或任何平台已验收。

更新技能时也执行上述检查。技能版本和应用依赖版本相互独立；更新技能不授权
升级 GOUI，开发时仍以应用锁定版本的源码和公开 API 为准。

安装不要求 Star、登录 GitHub、创建 Issue 或 PR，也不代用户执行这些操作。
