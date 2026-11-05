//go:build ignore

// Native sources are maintainer inputs; consumer builds use precompiled components.
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/platform/mobile_surface", map[string]string{
		"native/SurfaceUITests.m": `// 独立 XCTest target 操作已安装应用；旋转与 Home 事件走系统原生输入路径。
#import <XCTest/XCTest.h>
@interface ProbeUITests:XCTestCase
@end
@implementation ProbeUITests
- (void)testSurfaceLifecycle {
 self.continueAfterFailure=NO;
 XCUIApplication *app=[[XCUIApplication alloc]initWithBundleIdentifier:@"org.golanggui.mobilesurface"];
 [app activate];
 for(int i=0;i<8;i++){
  XCUIDevice.sharedDevice.orientation=(i%2)?UIDeviceOrientationPortrait:UIDeviceOrientationLandscapeLeft;
  [NSThread sleepForTimeInterval:1.8];
  XCTAssertEqual(app.state,XCUIApplicationStateRunningForeground);
 }
 for(int i=0;i<3;i++){
  [XCUIDevice.sharedDevice pressButton:XCUIDeviceButtonHome];[NSThread sleepForTimeInterval:1.0];
  [app activate];[NSThread sleepForTimeInterval:0.6];
 }
 XCUIDevice.sharedDevice.orientation=UIDeviceOrientationPortrait;
}
@end
`,
		"native/android.c": `// 测试包装仅提供私有数据路径；窗口/事件循环/呈现全部来自正式平台宿主。
#include <android/native_activity.h>
#include <stdlib.h>
#include <string.h>
extern void ANativeActivity_onCreate(ANativeActivity*,void*,size_t);
static char *path;
__attribute__((visibility("default"))) const char *probe_data_dir(void){return path;}
void probe_onCreate(ANativeActivity *a,void *saved,size_t size){if(!path)path=strdup(a->internalDataPath);ANativeActivity_onCreate(a,saved,size);}
`,
		"native/ios.m": `// 数据目录只供独立测试，不改变正式平台宿主的 UIKit 生命周期。
#import <Foundation/Foundation.h>
#include <pthread.h>
#include <string.h>
static char *path;
static pthread_once_t once=PTHREAD_ONCE_INIT;
static void Initialize(void){@autoreleasepool{path=strdup([[NSHomeDirectory() stringByAppendingPathComponent:@"Documents"] fileSystemRepresentation]);}}
__attribute__((visibility("default"))) const char *probe_data_dir(void){pthread_once(&once,Initialize);return path;}
`,
	})
}
