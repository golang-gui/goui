// 仅记录验收所需线程和沙箱；窗口、JNI 初始化和绘制均使用正式平台宿主。
#include <android/native_activity.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern void goui_android_create(ANativeActivity *, void *, size_t);
static char *directory;
static int32_t thread;
__attribute__((visibility("default"))) uintptr_t probe_directory(void) { return (uintptr_t)directory; }
__attribute__((visibility("default"))) int32_t probe_native_thread(void) { return thread; }
__attribute__((visibility("default"))) void ANativeActivity_onCreate(ANativeActivity *a, void *saved, size_t size) {
    if (!directory) { directory=strdup(a->internalDataPath); thread=gettid(); }
    goui_android_create(a,saved,size);
}
