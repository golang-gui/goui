// 在独立 XCTest UI target 中操作已安装的测试应用，不重新构建被测程序。
#import <XCTest/XCTest.h>
@interface ProbeUITests : XCTestCase
@end
@implementation ProbeUITests
- (void)testGoThreadAndLifecycle {
    self.continueAfterFailure = NO;
    XCUIApplication *app = [[XCUIApplication alloc] initWithBundleIdentifier:@"org.golanggui.mobilestart"];
    [app activate];
    XCUIElement *button = app.buttons[@"increment"];
    XCTAssertTrue([button waitForExistenceWithTimeout:15]);
    for (int i = 0; i < 10; i++) [button tap];
    NSPredicate *count = [NSPredicate predicateWithFormat:@"label CONTAINS %@", @"Count: 10"];
    XCTAssertTrue([[app.staticTexts matchingPredicate:count].firstMatch waitForExistenceWithTimeout:10]);
    for (int i = 0; i < 3; i++) {
        [[XCUIDevice sharedDevice] pressButton:XCUIDeviceButtonHome];
        [app activate];
        XCTAssertTrue([[app.staticTexts matchingPredicate:count].firstMatch waitForExistenceWithTimeout:10]);
    }
    [app.buttons[@"stop"] tap];
    NSPredicate *returned = [NSPredicate predicateWithFormat:@"label CONTAINS %@", @"returned: 1"];
    XCTAssertTrue([[app.staticTexts matchingPredicate:returned].firstMatch waitForExistenceWithTimeout:10]);
    [app.buttons[@"start"] tap];
    NSPredicate *main = [NSPredicate predicateWithFormat:@"label CONTAINS %@ AND label CONTAINS %@", @"Go main: 1", @"returned: 1"];
    XCTAssertTrue([[app.staticTexts matchingPredicate:main].firstMatch waitForExistenceWithTimeout:10]);
}
@end
