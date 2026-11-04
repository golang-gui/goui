// Optional platform graphics adapter. UIKit layer properties stay on main;
// each EAGL context and its GL objects belong to the calling GOUI thread.
#import <OpenGLES/EAGL.h>
#import <OpenGLES/ES3/gl.h>
#import <QuartzCore/CAEAGLLayer.h>
#include <dlfcn.h>

static _Atomic(uintptr_t) gl_layer;
static _Atomic(int) gl_allowed;
// Covers scratch work as well as drawable leases. Background entry closes this
// gate before returning; each completed binding drains submitted GPU commands.
static pthread_mutex_t gl_execution=PTHREAD_MUTEX_INITIALIZER;
typedef struct { void *context; GLuint scratch[3],drawable[3]; uint64_t generation; uintptr_t target; int bound; } GoUIGLContext;

static void HostGLSuspend(void) {
    pthread_mutex_lock(&gl_execution);
    atomic_store(&gl_allowed,0);
    pthread_mutex_unlock(&gl_execution);
}

static void DeleteFramebuffer(GLuint ids[3]) {
    if(ids[0])glDeleteFramebuffers(1,&ids[0]);
    if(ids[1])glDeleteRenderbuffers(2,&ids[1]);
    memset(ids,0,3*sizeof(GLuint));
}
static int CreateFramebuffer(GLuint ids[3],EAGLContext *context,CAEAGLLayer *layer) {
    DeleteFramebuffer(ids);glGenFramebuffers(1,&ids[0]);glBindFramebuffer(GL_FRAMEBUFFER,ids[0]);
    glGenRenderbuffers(2,&ids[1]);glBindRenderbuffer(GL_RENDERBUFFER,ids[1]);
    if(layer){if(![context renderbufferStorage:GL_RENDERBUFFER fromDrawable:layer])return -2;}
    else glRenderbufferStorage(GL_RENDERBUFFER,GL_RGBA8,1,1);
    GLint w=0,h=0;glGetRenderbufferParameteriv(GL_RENDERBUFFER,GL_RENDERBUFFER_WIDTH,&w);glGetRenderbufferParameteriv(GL_RENDERBUFFER,GL_RENDERBUFFER_HEIGHT,&h);
    glFramebufferRenderbuffer(GL_FRAMEBUFFER,GL_COLOR_ATTACHMENT0,GL_RENDERBUFFER,ids[1]);
    glBindRenderbuffer(GL_RENDERBUFFER,ids[2]);glRenderbufferStorage(GL_RENDERBUFFER,GL_DEPTH24_STENCIL8,w,h);
    glFramebufferRenderbuffer(GL_FRAMEBUFFER,GL_DEPTH_STENCIL_ATTACHMENT,GL_RENDERBUFFER,ids[2]);
    return glCheckFramebufferStatus(GL_FRAMEBUFFER)==GL_FRAMEBUFFER_COMPLETE?0:-3;
}
__attribute__((visibility("default"))) uintptr_t goui_host_gles_create(void) {
    @autoreleasepool {
        pthread_mutex_lock(&gl_execution);
        if(!atomic_load(&gl_allowed)){pthread_mutex_unlock(&gl_execution);return 0;}
        EAGLContext *context=[[EAGLContext alloc]initWithAPI:kEAGLRenderingAPIOpenGLES3];
        pthread_mutex_unlock(&gl_execution);
        if(!context)return 0;
        GoUIGLContext *c=calloc(1,sizeof(*c));if(!c)return 0;
        c->context=(__bridge_retained void*)context;return (uintptr_t)c;
    }
}
__attribute__((visibility("default"))) int32_t goui_host_gles_bind(uintptr_t object,uintptr_t target,uint64_t token) {
    @autoreleasepool {
        pthread_mutex_lock(&gl_execution);
        if(!atomic_load(&gl_allowed)){pthread_mutex_unlock(&gl_execution);return -1;}
        GoUIGLContext *c=(GoUIGLContext*)object;EAGLContext *context=(__bridge EAGLContext*)c->context;
        if(![EAGLContext setCurrentContext:context]){pthread_mutex_unlock(&gl_execution);return -2;}
        c->bound=1;
        if(!target){if(!c->scratch[0]){int result=CreateFramebuffer(c->scratch,context,nil);if(result)return result;}glBindFramebuffer(GL_FRAMEBUFFER,c->scratch[0]);}
        else {
            if(target!=(uintptr_t)(__bridge void*)view || token!=generation)return -1;
            CAEAGLLayer *layer=(__bridge CAEAGLLayer*)(void*)atomic_load(&gl_layer);if(!layer)return -1;
            if(c->generation!=token || c->target!=target){int result=CreateFramebuffer(c->drawable,context,layer);if(result)return result;}
            glBindFramebuffer(GL_FRAMEBUFFER,c->drawable[0]);
        }
        c->target=target;c->generation=token;return 0;
    }
}
__attribute__((visibility("default"))) int32_t goui_host_gles_clear(uintptr_t object) {
    GoUIGLContext *c=(GoUIGLContext*)object;
    if(!c->bound)return 0;
    // Conservative MVP: drain before releasing the gate so UIKit can enter
    // background without binding the context on another thread.
    glFinish();int32_t result=[EAGLContext setCurrentContext:nil]?0:-2;
    c->bound=0;pthread_mutex_unlock(&gl_execution);return result;
}
__attribute__((visibility("default"))) int32_t goui_host_gles_swap(uintptr_t object) {
    GoUIGLContext *c=(GoUIGLContext*)object;
    if(!c->target || c->target!=(uintptr_t)(__bridge void*)view || c->generation!=generation)return -1;
    glBindRenderbuffer(GL_RENDERBUFFER,c->drawable[1]);
    return [(__bridge EAGLContext*)c->context presentRenderbuffer:GL_RENDERBUFFER]?0:-2;
}
__attribute__((visibility("default"))) int32_t goui_host_gles_interval(uintptr_t object,int32_t interval) {
    (void)object;(void)interval;return -4; // EAGL has no swap-interval request API.
}
__attribute__((visibility("default"))) uintptr_t goui_host_gles_proc(const char *name) {return (uintptr_t)dlsym(RTLD_DEFAULT,name);}
__attribute__((visibility("default"))) void goui_host_gles_destroy(uintptr_t object) {
    @autoreleasepool {
        GoUIGLContext *c=(GoUIGLContext*)object;
        pthread_mutex_lock(&gl_execution);
        if(atomic_load(&gl_allowed)){
            [EAGLContext setCurrentContext:(__bridge EAGLContext*)c->context];
            DeleteFramebuffer(c->scratch);DeleteFramebuffer(c->drawable);glFinish();[EAGLContext setCurrentContext:nil];
        }
        CFRelease(c->context);free(c);
        pthread_mutex_unlock(&gl_execution);
    }
}

@interface GoUIGLESView:UIView
@end
@implementation GoUIGLESView
+ (Class)layerClass {return CAEAGLLayer.class;}
@end
