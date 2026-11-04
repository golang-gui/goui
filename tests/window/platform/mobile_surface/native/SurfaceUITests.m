// 独立 XCTest target 操作已安装应用；旋转与 Home 事件走系统原生输入路径。
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
