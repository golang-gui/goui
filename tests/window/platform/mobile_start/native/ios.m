// UIKit 操作例程，原生控件仅用于确认系统循环和 Go 线程分别保持响应。
#import <UIKit/UIKit.h>
#import <Foundation/Foundation.h>
#include "probe.h"

static void Save(void) {
    NSString *dir = [NSHomeDirectory() stringByAppendingPathComponent:@"Documents"];
    [[NSFileManager defaultManager] createDirectoryAtPath:dir withIntermediateDirectories:YES attributes:nil error:nil];
    FILE *f = fopen([[dir stringByAppendingPathComponent:@"go-state.json"] fileSystemRepresentation], "w");
    if (f) { ProbeWrite(f, "tick"); fclose(f); }
}
@interface ProbeController : UIViewController
@property(nonatomic,strong) UILabel *status;
- (void)increment;
- (void)stop;
- (void)start;
@end
@implementation ProbeController
- (void)viewDidLoad {
    [super viewDidLoad];
    self.view.backgroundColor = UIColor.systemBackgroundColor;
    self.status = [[UILabel alloc] initWithFrame:CGRectMake(20, 180, 350, 90)];
    self.status.numberOfLines = 3;
    [self.view addSubview:self.status];
    UIButton *button = [UIButton buttonWithType:UIButtonTypeSystem];
    button.frame = CGRectMake(20, 300, 300, 60);
    [button setTitle:@"Increment on Go thread" forState:UIControlStateNormal];
    [button addTarget:self action:@selector(increment) forControlEvents:UIControlEventTouchUpInside];
    button.accessibilityIdentifier = @"increment";
    [self.view addSubview:button];
    UIButton *stop = [UIButton buttonWithType:UIButtonTypeSystem];
    stop.frame = CGRectMake(20, 370, 300, 60);
    [stop setTitle:@"Stop Go loop (test only)" forState:UIControlStateNormal];
    [stop addTarget:self action:@selector(stop) forControlEvents:UIControlEventTouchUpInside];
    stop.accessibilityIdentifier = @"stop"; [self.view addSubview:stop];
    UIButton *start = [UIButton buttonWithType:UIButtonTypeSystem];
    start.frame = CGRectMake(20, 440, 300, 60);
    [start setTitle:@"Request startup again" forState:UIControlStateNormal];
    [start addTarget:self action:@selector(start) forControlEvents:UIControlEventTouchUpInside];
    start.accessibilityIdentifier = @"start"; [self.view addSubview:start];
    __weak ProbeController *weakSelf = self;
    [NSTimer scheduledTimerWithTimeInterval:0.1 repeats:YES block:^(NSTimer *timer) {
        ProbeController *controller = weakSelf;
        if (!controller) { [timer invalidate]; return; }
        controller.status.text = [NSString stringWithFormat:@"Go main: %d / events: %d\nCount: %d / thread failures: %d / returned: %d",
            atomic_load(&probe_mains), atomic_load(&probe_events), atomic_load(&probe_taps), atomic_load(&probe_failures), atomic_load(&probe_returned)];
        Save();
    }];
}
- (void)increment { ProbeSend(1); }
- (void)stop { ProbeSend(99); }
- (void)start { ProbeStart(); }
@end
@interface ProbeScene : UIResponder<UIWindowSceneDelegate>
@property(nonatomic,strong) UIWindow *window;
@end
@implementation ProbeScene
- (void)scene:(UIScene *)scene willConnectToSession:(UISceneSession *)session options:(UISceneConnectionOptions *)options {
    (void)session; (void)options;
    self.window = [[UIWindow alloc] initWithWindowScene:(UIWindowScene *)scene];
    self.window.rootViewController = [[ProbeController alloc] init];
    [self.window makeKeyAndVisible];
}
- (void)sceneDidBecomeActive:(UIScene *)scene { (void)scene; ProbeStart(); ProbeSend(2); }
- (void)sceneDidEnterBackground:(UIScene *)scene { (void)scene; ProbeSend(3); Save(); }
- (void)scene:(UIScene *)scene openURLContexts:(NSSet<UIOpenURLContext *> *)contexts {
    (void)scene;
    for (UIOpenURLContext *context in contexts) {
        NSString *action = context.URL.host;
        if ([action isEqualToString:@"increment"]) ProbeSend(1);
        if ([action isEqualToString:@"stop"]) ProbeSend(99);
        if ([action isEqualToString:@"start"]) ProbeStart();
    }
}
@end
@interface ProbeDelegate : UIResponder<UIApplicationDelegate>
@end
@implementation ProbeDelegate
- (BOOL)application:(UIApplication *)app didFinishLaunchingWithOptions:(NSDictionary *)options {
    (void)app; (void)options;
    probe_ui = pthread_self(); ProbeStart(); ProbeStart(); Save();
    return YES;
}
@end
int main(int argc, char **argv) {
    @autoreleasepool { return UIApplicationMain(argc, argv, nil, NSStringFromClass([ProbeDelegate class])); }
}
