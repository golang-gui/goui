// 原生验收状态，仅用于线程/生命周期测试；不属于分发给应用的预编译基座。
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
