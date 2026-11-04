// Private platform-host ABI. This component is separate from runtime/cgo.
#include <stdint.h>
#include <stddef.h>
#include <stdatomic.h>

enum { GOUI_TARGET=1, GOUI_LOST, GOUI_VISIBLE, GOUI_FOCUS, GOUI_REDRAW };
typedef struct {
    int32_t kind, width, height;
    float scale;
    uintptr_t handle;
    uint64_t generation;
} GoUIHostEvent;
_Static_assert(sizeof(GoUIHostEvent)==32 && offsetof(GoUIHostEvent,handle)==16 && offsetof(GoUIHostEvent,generation)==24,"mobile host ABI");
typedef uintptr_t (*GoUIHostCallback)(const GoUIHostEvent *);
extern int goui_run(void);
