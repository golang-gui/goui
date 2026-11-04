// 独立 XCTest target：系统旋转与 Home/activate，不调用 GUI 语义捷径。
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
