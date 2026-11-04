// 测试包装仅提供私有数据路径；窗口/事件循环/呈现全部来自正式平台宿主。
#include <android/native_activity.h>
#include <stdlib.h>
#include <string.h>
extern void goui_android_create(ANativeActivity*,void*,size_t);
static char *path;
__attribute__((visibility("default"))) const char *probe_data_dir(void){return path;}
void ANativeActivity_onCreate(ANativeActivity *a,void *saved,size_t size){if(!path)path=strdup(a->internalDataPath);goui_android_create(a,saved,size);}
