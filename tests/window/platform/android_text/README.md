# Android 系统排版能力验证

目的：验证公开系统排版能否满足 GOUI 现有 TextLayout，特别是 shaping Cluster。
这是能力实验，不是 androidtext 后端；候选映射不成立必须报告，不算整体支持。

环境：Android ARM64、API 29 及以上；已验证的移动 FFI/基座分支、Go 1.25.8、
NativeActivity 正式宿主和 Software。程序含 Java 测试 helper，所以测试 APK
hasCode=true。Go 用户代码不使用 import C，不访问系统私有文字库或 DevServer。

维护者准备：可使用 NDK 编译测试包装、Java 11 的 jdk.compiler 和 D8 制作 Dex；
这些步骤与用户消费构建分开。Python 3 无第三方模块依赖，但参考生成需要
维护者系统安装的公共 HarfBuzz，以及含 ffi 连字/希伯来文/阿拉伯文的字体
（例如 DejaVuSans.ttf）。字体只复制进本次临时 APK，不向仓库增加字体副本。

构建：python3 build.py /仓库外/config.json。JSON 中全部路径由维护者显式提供：

```json
{
  "output": "/tmp/唯一的本轮目录",
  "font": "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
  "java": "/usr/bin/java",
  "androidJar": "/工具/android.jar",
  "d8": "/工具/d8.jar",
  "cc": "/工具/android-clang",
  "cgo": "/源码/cgo",
  "purego": "/源码/purego",
  "gouiBase": "/工具/goui-base",
  "go": "/工具/go1.25.8/bin/go",
  "driver": "/工具/go-mobile",
  "linker": "/工具/goui-link",
  "base": "/组件/base.json",
  "inputs": "/组件/inputs.json",
  "tools": "/组件/tools.json",
  "host": "/组件/正式Android宿主.o",
  "consumerPath": "/只含消费构建所需工具的目录",
  "aapt2": "/工具/aapt2",
  "signer": "/工具/apksign",
  "key": "/仅用于本次实验的私钥.pem",
  "cert": "/仅用于本次实验的证书.pem",
  "apksigner": "/独立校验工具/apksigner.jar"
}
```

host 是 platform/mobile/native/android.c 以
-DANativeActivity_onCreate=goui_android_create 单独预编译的正式宿主；不把本例
helper 并入 runtime/cgo 基座。签名仅用临时测试身份，不修改系统安全配置。
build-receipt.json 和 consume.log 区分维护者编译与无原生编译器消费。

启动：安装输出目录的 probe.apk，然后启动
org.golanggui.androidtext/android.app.NativeActivity。初始画面是浅色背景的多语言
文字，包括 Latin、中、日、韩、阿拉伯、希伯来、印地语和 emoji；下方给出
core failures 与 cluster candidate failures。整个图片由系统 Canvas 生成，再
通过 GOUI Software Painter/graphics.Image 呈现，不是主线程 TextView 截图。

操作与逐项预期：

1. 等待画面出现，读取私有状态：
   adb shell run-as org.golanggui.androidtext cat files/report.json
   以及 files/go.json。frames>0、owner=nativeThread，workerThread=true。
2. 检查每项 pass，不能只检查进程存活。coreFailures 应为 0；失败要保留具体
   输入。candidateFailures 可以非零，它表示候选映射不能满足 shaping 契约，
   不能解释为 Android 文字本身错误，或据此直接实现生产 TextLayout。
3. 独立字体的 HarfBuzz cluster 通过字体 SHA256 固定，比较 cursor 合法位置及
   非零 advance 分组。ffi 必须是一个参考 glyph。差异保留在 clusterObservations；
   原生 ffi 与同字体 U+FB03 的像素应精确相等，确认 Android 实际形成连字。
   不将 UTF-16、字素、glyph 或合法编辑位置直接冒充 shaping cluster。
4. 检查组合字符/代理对/ZWJ/旗帜/印度文字、RTL 运行、行范围/末尾换行、Tab、
   无绘制占位和相邻占位、独立 Layout 的颜色/度量隔离。
5. 区域栅格化在 1x/2x 对比完整图像相同位置，像素要求精确相等，包含负原点及
   透明空白；同一系统、字体和渲染路径不引入跨平台抗锯齿容差。RGBA 内存样本
   的源色 ARGB=0x80402010，预乘 RGBA 应为 (32,16,8,128)。
   测试设置 LINEAR_TEXT_FLAG 验证小数度量，报告同时记录 hinted 对照；该标志
   的 hinting/cache 取舍未做性能验收，不能直接当作生产后端的最终配置。
6. 观察真实截图与 files/text.png 的多语言字形，检查缺字、顺序、裁剪；不使用
   跨平台统一字体金图。字体回退依赖系统；本例不要求各系统字形完全相同。

退出：系统返回/强制停止本测试应用；复位：清除本包数据或卸载。只使用专用
模拟器和 ADB 端口，验收后清理本次设备，不接管用户其他设备。实验字体、APK、
JSON、截图和日志放在仓库外；本例 helper 的进程级 Java 引用随测试进程结束。
正式 JNI 后端仍需完整的对象释放、异常转换、状态失效与 TextLayout 测试。
