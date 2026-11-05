//go:build ignore

// Native sources are maintainer inputs; consumer builds use precompiled components.
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/platform/android_gui", map[string]string{
		"native/android.c": `// 仅记录验收所需线程和沙箱；窗口、JNI 初始化和绘制均使用正式平台宿主。
#include <android/native_activity.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern void ANativeActivity_onCreate(ANativeActivity *, void *, size_t);
static char *directory;
static int32_t thread;
__attribute__((visibility("default"))) uintptr_t probe_directory(void) { return (uintptr_t)directory; }
__attribute__((visibility("default"))) int32_t probe_native_thread(void) { return thread; }
__attribute__((visibility("default"))) void probe_onCreate(ANativeActivity *a, void *saved, size_t size) {
    if (!directory) { directory=strdup(a->internalDataPath); thread=gettid(); }
    ANativeActivity_onCreate(a,saved,size);
}
`,
	})
}
