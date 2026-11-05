//go:build ignore

// Native sources are maintainer inputs; consumer builds use precompiled components.
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/platform/mobile_paint", map[string]string{
		"native/PaintUITests.m": `// 独立 XCTest target：系统旋转与 Home/activate，不调用 GUI 语义捷径。
// 由宿主脚本同时读取 paint.json，断言尺寸、nonce、后台停绘及恢复。
#import <XCTest/XCTest.h>

@interface ProbeUITests : XCTestCase
@end

@implementation ProbeUITests
- (void)testPaintLifecycle {
    self.continueAfterFailure = NO;
    XCUIApplication *app = [[XCUIApplication alloc]
        initWithBundleIdentifier:@"org.golanggui.mobilepaint"];
    [app activate];
    for (int i = 0; i < 8; i++) {
        XCUIDevice.sharedDevice.orientation = i % 2
            ? UIDeviceOrientationPortrait : UIDeviceOrientationLandscapeLeft;
        // 给宿主留出独立截图时间；宿主还须核对截图前后的实际尺寸一致。
        [NSThread sleepForTimeInterval:4.0];
        XCTAssertEqual(app.state, XCUIApplicationStateRunningForeground);
    }
    for (int i = 0; i < 3; i++) {
        [XCUIDevice.sharedDevice pressButton:XCUIDeviceButtonHome];
        // Home 动画结束后仍须保留至少一秒真实后台观察时间。
        [NSThread sleepForTimeInterval:3.0];
        [app activate];
        [NSThread sleepForTimeInterval:0.8];
    }
    XCUIDevice.sharedDevice.orientation = UIDeviceOrientationPortrait;
}
@end
`,
		"native/android.c": `// 测试仅公开沙箱目录与线程事实；生命周期和绘制使用正式 Android 宿主。
#include <android/native_activity.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <pthread.h>
#include <stddef.h>
_Static_assert(sizeof(ANativeActivity)==80,"ANativeActivity ARM64 layout");
_Static_assert(offsetof(ANativeActivity, sdkVersion)==48,"ANativeActivity SDK offset");
_Static_assert(sizeof(ANativeActivityCallbacks)==128,"ANativeActivityCallbacks layout");
_Static_assert(sizeof(ANativeWindow_Buffer)==48,"ANativeWindow_Buffer layout");
_Static_assert(offsetof(struct JNINativeInterface,CallIntMethodA)==51*sizeof(void*),"JNI CallIntMethodA index");
extern void ANativeActivity_onCreate(ANativeActivity *,void *,size_t);
__attribute__((visibility("default"))) int probe_use_gles(void){return GOUI_GLES;}
static char *path;
__attribute__((visibility("default"))) const char *probe_data_dir(void){return path;}
__attribute__((visibility("default"))) uintptr_t probe_thread(void){return (uintptr_t)pthread_self();}
__attribute__((visibility("default"))) int probe_is_main(void){return gettid()==getpid();}
__attribute__((visibility("default"))) void probe_onCreate(ANativeActivity *a,void *saved,size_t size){
    if(!path){
        path=strdup(a->internalDataPath);
        char file[4096];int n=snprintf(file,sizeof(file),"%s/go.stderr",path);
        if(n>0 && n<(int)sizeof(file)){int fd=open(file,O_CREAT|O_WRONLY|O_TRUNC,0600);if(fd>=0){dup2(fd,STDERR_FILENO);close(fd);}}
    }
    ANativeActivity_onCreate(a,saved,size);
}
`,
		"native/ios.m": `// 测试仅公开沙箱目录、线程与已加载库检查，不替代正式 UIKit 宿主或文字实现。
#import <Foundation/Foundation.h>
#include <dlfcn.h>
#include <pthread.h>
#include <string.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
__attribute__((visibility("default"))) int probe_use_gles(void){return GOUI_GLES;}
static char *path;
static pthread_once_t once=PTHREAD_ONCE_INIT;
static void Initialize(void){@autoreleasepool{
    NSString *directory=[NSHomeDirectory() stringByAppendingPathComponent:@"Documents"];
    [[NSFileManager defaultManager]createDirectoryAtPath:directory withIntermediateDirectories:YES attributes:nil error:nil];
    path=strdup(directory.fileSystemRepresentation);
    int fd=open([[directory stringByAppendingPathComponent:@"go.stderr"]fileSystemRepresentation],O_CREAT|O_WRONLY|O_TRUNC,0600);
    if(fd>=0){dup2(fd,STDERR_FILENO);close(fd);}
}}
__attribute__((visibility("default"))) const char *probe_data_dir(void){pthread_once(&once,Initialize);return path;}
__attribute__((visibility("default"))) uintptr_t probe_thread(void){return (uintptr_t)pthread_self();}
__attribute__((visibility("default"))) int probe_is_main(void){return pthread_main_np();}
__attribute__((visibility("default"))) int probe_desktop_loaded(void){
    int result=0;
    const char *paths[]={"/System/Library/Frameworks/AppKit.framework/AppKit","/System/Library/Frameworks/OpenGL.framework/OpenGL"};
    for(int i=0;i<2;i++){void *h=dlopen(paths[i],RTLD_NOW|RTLD_NOLOAD);if(h){dlclose(h);result|=1<<i;}}
    return result;
}
`,
	})
}
