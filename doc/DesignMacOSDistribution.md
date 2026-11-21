# macOS 跨平台签名与公证

## 目标与范围

`cmd/goui` 在 Linux、Windows、macOS 上消费现有 darwin/arm64 二进制，生成
可签名、公证和装订票据的 `.app`，最终输出保留 Unix 权限的 ZIP。不依赖宿主
Xcode、codesign、notarytool 或 stapler；macOS 现有钥匙串签名入口仍保留。
首期仅支持 GOUI 当前的单 Mach-O 可执行文件与普通资源；嵌套代码、Framework、
插件和 Universal Bundle 必须明确拒绝，不能当作普通文件封装后宣称完整签名。

构建、签名格式验证、公证 Accepted 和真实桌面运行是不同证据。支持承诺须以
macOS 原生验签、票据与 Gatekeeper 检查及最终解压产物的窗口操作补齐。

## 命令与配置

```toml
[macos.signing]
pkcs12 = "/private/developer-id.p12"
password_env = "MACOS_SIGN_PASSWORD"

[macos.notary]
private_key = "/private/AuthKey.p8"
key_id = "KEYID"
issuer_id = "ISSUER-UUID"
```

PKCS#12 包含私钥、Developer ID Application 证书和中间链。签名输入与 Notary
API 私钥分别加载，不上传私钥，不将密码或 JWT 写入 argv、收据或错误日志。
配置与已有 `macos.signing_identity` 二选一；不隐式导出钥匙串或寻找其他身份。
PKCS#12 入口用于正式 Developer ID 分发，始终开启 Hardened Runtime 和 Apple
安全时间戳；时间戳失败即签名失败，不自动降级到开发签名。沙箱仍是独立可选项。

```sh
goui build -target darwin/arm64
goui package -target darwin/arm64 -format app
goui notarize -target darwin/arm64 -app "dist/darwin-arm64/Notes.app" -o notary.json -wait
goui notarize -target darwin/arm64 -submission <ID> -o notary.json -wait
goui staple -target darwin/arm64 -app "dist/darwin-arm64/Notes.app"
goui archive -target darwin/arm64 -app "dist/darwin-arm64/Notes.app" -o Notes.zip
```

- `build` 仍只编译；`package` 仍只消费二进制并按明确配置签名，不上传应用。
- `notarize` 显式上传临时 ZIP；创建 submission 后立即保存 ID，再上传应用。
  `-submission` 只续查已有任务；`-wait` 使用有界轮询，超时保留 ID，不重新上传。
  收据只保存任务身份、上传摘要、状态及结果，不保存临时 S3 凭据。
- `staple` 按实际 CodeDirectory 的 cdhash 查询 Apple 票据，验证签名及目标绑定，
  成功后原子写入 `Contents/CodeResources`。不改写 Mach-O 或资源签名。
- `archive` 只消费已有 `.app`，在 ZIP 中显式写 Unix 权限；不会重新签名，也不
  删除已装订票据。默认拒绝覆盖已有输出，临时目录或归档由本次调用负责清理。

Notary API、Apple 时间戳和票据查询需要网络；本地组包、哈希与验签无需网络。
公证收据中的 Accepted 不等于本地票据存在；最终 ZIP 应在装订后生成。

## 签名实现

共用 iOS 已有的 PKCS#12、Mach-O、CodeDirectory、CMS 与 Entitlements 编码能力，
保留 iOS profile、应用授权和 macOS Developer ID 策略的边界。证书校验覆盖有效期、
公私钥匹配、Apple 信任链、Developer ID Application 扩展和 Team ID。

macOS 单独生成 `Contents/_CodeSignature/CodeResources`，密封普通资源；Info.plist
及主程序由签名专用槽和代码页保护。资源清单按明确 Bundle 路径生成，不把 iOS
根目录规则直接套到 Contents。保留规定的票据位置；未知代码或符号链接不静默跟随。
签名过程中不得改动 load commands 和签名区以外的可执行字节。

CMS 安全时间戳绑定实际签名值，使用 RFC 3161；验证响应摘要、nonce、时间戳
签名和 Apple 信任链。主程序签名同时写 Hardened Runtime 与签名要求。
验签覆盖代码页、特殊槽、CMS、证书、资源清单、缺失／新增／修改资源及权限声明。
仅消化已理解且值为 ASN.1 NULL 的 Apple 证书角色 critical 扩展，其余未知
critical 扩展继续拒绝；不修改原始证书。离线验签不代替系统撤销策略与 Gatekeeper。

## 公证与装订

公证使用 Apple 官方 Notary REST API：本地签 ES256 JWT，申请 submission，
使用返回的短期凭据上传 S3，按 ID 获取状态和日志。网络失败报告所在阶段与已知 ID，
不打印带凭据的 URL。Invalid 返回失败并保留诊断；未知状态不能伪造 Accepted。

上传对象键按 S3 SigV4 规则进行 UTF-8 字节编码，保留路径分隔斜线，将冒号、
加号和空格等编码为百分号序列。实际请求路径与签名的 canonical URI 使用同一份
编码，不依赖 Go URL 对保留字符的默认处理，也不在签名时重复编码。

票据交付与票据格式没有 Apple 公共稳定 API 承诺，按独立边界处理：参考 Apple
开源的 Bundle 票据位置及 rcodesign 的 CloudKit 查询；仅接受实际支持的票据版本、
Apple 签名和当前 cdhash。未知版本、错误目标、损坏或非 Apple 签名均拒绝装订。
协议变更须有原生对照和回归，不能仅根据 HTTP 成功或文件存在宣称票据有效。

## 验证

无桌面测试留 `cmd/goui` 已有签名／组包主题，必要的新公证协议主题独立成文件。
使用合成证书、时间戳服务和 HTTP 替身验证失败路径、取消、续查、错误凭据、
篡改、目标绑定和失败不发布。合成材料不作为 Apple 正式身份或分发验收证据。
既有 iOS 签名测试必须保留；原生签名验证留既有平台测试或独立集成入口。

以 Linux／Windows 生成的产物在 macOS 通过 `codesign --verify --strict`、
`stapler validate`、`spctl --assess`，核对 runtime、时间戳、Team ID 和资源保护，
再检查正式 ZIP 解压后的权限、启动与窗口操作。未执行项明确记录，不将交叉编译
或公证 Accepted 当作全部验收通过。

## 参考

- [Apple Notary REST API](https://developer.apple.com/documentation/notaryapi/submitting-software-for-notarization-over-the-web)：官方上传、认证、任务与续查契约。
- [Apple 公证要求](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)：Developer ID、runtime、安全时间戳。
- [Apple 自定义公证流程](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)：先装订 `.app`，再生成最终 ZIP。
- [Apple Security notarization.cpp](https://github.com/apple-oss-distributions/Security/blob/main/OSX/libsecurity_codesigning/lib/notarization.cpp)：Bundle 票据位置的源码行为。
- [rcodesign 0.29.0](https://gregoryszorc.com/docs/apple-codesign/stable/)：跨平台实现参考，不引入 Rust 工具依赖，也不据此承诺完整复刻 Apple 工具。
- [Wails v3.1.0 internal/notaryticket](https://github.com/wailsapp/wails/tree/v3.1.0/v3/internal/notaryticket)：s8ch/v1、g8tk/v2 的字段与 ECDSA 签名范围参考；GOUI 独立校验 Apple 证书角色、链和当前代码摘要。
- [AWS Smithy Go v1.22.2 路径编码](https://github.com/aws/smithy-go/blob/v1.22.2/encoding/httpbinding/path_replace.go)：复用 SDK 的 UTF-8 百分号编码；测试以明确 URI 字面量和独立 SigV4 验算验证请求与签名一致。
