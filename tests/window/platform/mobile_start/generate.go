//go:build ignore

// Native sources are maintainer inputs; consumer builds use precompiled components.
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/platform/mobile_start", map[string]string{
		"native/ProbeUITests.m": `// 在独立 XCTest UI target 中操作已安装的测试应用，不重新构建被测程序。
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
`,
		"native/android.c": `// NativeActivity 操作例程，原生绘制仅用于确认系统循环仍在运行。
#include <android/native_activity.h>
#include <android/native_window.h>
#include <android/input.h>
#include <android/looper.h>
#include <sys/timerfd.h>
#include <unistd.h>
#include <stdlib.h>
#include "probe.h"

typedef struct {
    ANativeActivity *activity;
    ANativeWindow *window;
    AInputQueue *input;
    ALooper *looper;
    int timer;
} Host;

static void Save(Host *h) {
    char path[2048];
    snprintf(path, sizeof(path), "%s/go-state.json", h->activity->internalDataPath);
    FILE *f = fopen(path, "w");
    if (f) { ProbeWrite(f, "tick"); fclose(f); }
}
static void Draw(Host *h) {
    if (!h->window) return;
    ANativeWindow_Buffer b;
    ANativeWindow_setBuffersGeometry(h->window, 0, 0, WINDOW_FORMAT_RGBA_8888);
    if (ANativeWindow_lock(h->window, &b, NULL) != 0) return;
    uint32_t *pixels = b.bits;
    for (int y = 0; y < b.height; y++) for (int x = 0; x < b.width; x++) {
        uint32_t color = 0xfff8f4ee;
        if (y > b.height / 3 && y < b.height * 2 / 3 && x > b.width / 8 && x < b.width * 7 / 8)
            color = (atomic_load(&probe_taps) & 1) ? 0xff80ad46 : 0xffdc9b38;
        pixels[y * b.stride + x] = color;
    }
    ANativeWindow_unlockAndPost(h->window);
}
static int Tick(int fd, int events, void *data) {
    (void)events;
    uint64_t count;
    if (read(fd, &count, sizeof(count)) > 0) { Draw(data); Save(data); }
    return 1;
}
static int Input(int fd, int events, void *data) {
    (void)fd; (void)events;
    Host *h = data;
    AInputEvent *event;
    while (AInputQueue_getEvent(h->input, &event) >= 0) {
        if (AInputQueue_preDispatchEvent(h->input, event)) continue;
        int handled = AInputEvent_getType(event) == AINPUT_EVENT_TYPE_MOTION;
        if (handled && (AMotionEvent_getAction(event) & AMOTION_EVENT_ACTION_MASK) == AMOTION_EVENT_ACTION_UP) ProbeSend(1);
        AInputQueue_finishEvent(h->input, event, handled);
    }
    return 1;
}
static void Resume(ANativeActivity *a) { (void)a; ProbeStart(); ProbeSend(2); }
static void Stop(ANativeActivity *a) { (void)a; ProbeSend(3); }
static void Window(ANativeActivity *a, ANativeWindow *w) {
    Host *h = a->instance;
    ANativeWindow_acquire(w); h->window = w; Draw(h);
}
static void Redraw(ANativeActivity *a, ANativeWindow *w) { (void)w; Draw(a->instance); }
static void LoseWindow(ANativeActivity *a, ANativeWindow *w) {
    Host *h = a->instance;
    if (h->window == w) { h->window = NULL; ANativeWindow_release(w); }
}
static void Queue(ANativeActivity *a, AInputQueue *q) {
    Host *h = a->instance; h->input = q;
    AInputQueue_attachLooper(q, h->looper, ALOOPER_POLL_CALLBACK, Input, h);
}
static void LoseQueue(ANativeActivity *a, AInputQueue *q) {
    Host *h = a->instance; AInputQueue_detachLooper(q); h->input = NULL;
}
static void Destroy(ANativeActivity *a) {
    Host *h = a->instance;
    Save(h);
    ALooper_removeFd(h->looper, h->timer); close(h->timer);
    if (h->input) AInputQueue_detachLooper(h->input);
    if (h->window) ANativeWindow_release(h->window);
    a->instance = NULL; free(h);
}
__attribute__((visibility("default"))) void probe_onCreate(ANativeActivity *a, void *saved, size_t size) {
    (void)saved; (void)size;
    probe_ui = pthread_self();
    Host *h = calloc(1, sizeof(*h)); a->instance = h; h->activity = a;
    h->looper = ALooper_forThread();
    h->timer = timerfd_create(CLOCK_MONOTONIC, TFD_NONBLOCK | TFD_CLOEXEC);
    if (!h->looper || h->timer < 0) abort();
    struct itimerspec interval = {{0, 100000000}, {0, 100000000}};
    timerfd_settime(h->timer, 0, &interval, NULL);
    ALooper_addFd(h->looper, h->timer, ALOOPER_POLL_CALLBACK, ALOOPER_EVENT_INPUT, Tick, h);
    ANativeActivityCallbacks *c = a->callbacks;
    c->onResume = Resume; c->onStop = Stop; c->onDestroy = Destroy;
    c->onNativeWindowCreated = Window; c->onNativeWindowDestroyed = LoseWindow;
    c->onNativeWindowRedrawNeeded = Redraw;
    c->onInputQueueCreated = Queue; c->onInputQueueDestroyed = LoseQueue;
    ProbeStart(); ProbeStart(); Save(h);
}
`,
		"native/ios.m": `// UIKit 操作例程，原生控件仅用于确认系统循环和 Go 线程分别保持响应。
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
`,
		"native/probe.h": `// 原生验收状态，仅用于线程/生命周期测试；不属于分发给应用的预编译基座。
#include <stdint.h>
#include <stdatomic.h>
#include <pthread.h>
#include <time.h>
#include <stdio.h>

extern int goui_run(void);
static _Atomic(pthread_t) probe_ui;
static _Atomic(uintptr_t) probe_callback;
static _Atomic(uint64_t) probe_go_thread;
static _Atomic(int) probe_mains, probe_events, probe_taps, probe_active;
static _Atomic(int) probe_background, probe_failures, probe_returned;
static int probe_entries;
static double probe_entry_ms;

__attribute__((visibility("default"))) int probe_is_ui(void) {
    return pthread_equal(pthread_self(), atomic_load(&probe_ui));
}
__attribute__((visibility("default"))) void probe_publish(uintptr_t callback, uint64_t thread) {
    if (probe_is_ui()) atomic_fetch_add(&probe_failures, 1);
    atomic_store(&probe_go_thread, thread);
    atomic_fetch_add(&probe_mains, 1);
    atomic_store(&probe_callback, callback);
}
__attribute__((visibility("default"))) void probe_record(int64_t code, uint64_t thread) {
    if (probe_is_ui() || thread != atomic_load(&probe_go_thread)) atomic_fetch_add(&probe_failures, 1);
    atomic_fetch_add(&probe_events, 1);
    if (code == 1) atomic_fetch_add(&probe_taps, 1);
    if (code == 2) atomic_fetch_add(&probe_active, 1);
    if (code == 3) atomic_fetch_add(&probe_background, 1);
    if (code == 100) atomic_fetch_add(&probe_returned, 1);
}
static void ProbeSend(int64_t code) {
    uintptr_t address = atomic_load(&probe_callback);
    if (address) ((int64_t (*)(int64_t))address)(code);
}
static void ProbeStart(void) {
    struct timespec before, after;
    clock_gettime(CLOCK_MONOTONIC, &before);
    int result = goui_run();
    clock_gettime(CLOCK_MONOTONIC, &after);
    if (result != 0) atomic_fetch_add(&probe_failures, 1);
    probe_entries++;
    double ms = (after.tv_sec - before.tv_sec) * 1000.0 + (after.tv_nsec - before.tv_nsec) / 1e6;
    if (ms > probe_entry_ms) probe_entry_ms = ms;
}
static void ProbeWrite(FILE *f, const char *event) {
    fprintf(f, "{\"event\":\"%s\",\"mainCalls\":%d,\"events\":%d,\"taps\":%d,\"active\":%d,\"background\":%d,\"threadFailures\":%d,\"returned\":%d,\"entries\":%d,\"entryMs\":%.3f}\n",
        event, atomic_load(&probe_mains), atomic_load(&probe_events), atomic_load(&probe_taps),
        atomic_load(&probe_active), atomic_load(&probe_background), atomic_load(&probe_failures),
        atomic_load(&probe_returned), probe_entries, probe_entry_ms);
}
`,
	})
}
