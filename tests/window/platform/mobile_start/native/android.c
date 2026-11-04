// NativeActivity 操作例程，原生绘制仅用于确认系统循环仍在运行。
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
__attribute__((visibility("default"))) void ANativeActivity_onCreate(ANativeActivity *a, void *saved, size_t size) {
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
