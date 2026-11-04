// Optional platform graphics adapter, compiled with GOUI_GLES. Not runtime base.
#include <EGL/egl.h>
#include <GLES3/gl3.h>
#include <dlfcn.h>

static EGLDisplay gl_display=EGL_NO_DISPLAY;
static EGLConfig gl_config;
static EGLSurface gl_surface=EGL_NO_SURFACE;
static EGLint gl_visual;

static void HostGLInit(void) {
    if(gl_display!=EGL_NO_DISPLAY)return;
    gl_display=eglGetDisplay(EGL_DEFAULT_DISPLAY);
    EGLint count=0;
    const EGLint attrs[]={EGL_SURFACE_TYPE,EGL_WINDOW_BIT|EGL_PBUFFER_BIT,EGL_RENDERABLE_TYPE,0x40,
        EGL_RED_SIZE,8,EGL_GREEN_SIZE,8,EGL_BLUE_SIZE,8,EGL_ALPHA_SIZE,8,
        EGL_DEPTH_SIZE,24,EGL_STENCIL_SIZE,8,EGL_NONE};
    if(gl_display==EGL_NO_DISPLAY || !eglInitialize(gl_display,NULL,NULL) ||
       !eglChooseConfig(gl_display,attrs,&gl_config,1,&count) || count!=1 ||
       !eglGetConfigAttrib(gl_display,gl_config,EGL_NATIVE_VISUAL_ID,&gl_visual))abort();
}

// Runs on the native UI thread after synchronous target revocation. No GOUI
// thread has the old surface current when it is destroyed here.
static void HostGLTarget(ANativeWindow *next) {
    if(gl_surface!=EGL_NO_SURFACE){eglDestroySurface(gl_display,gl_surface);gl_surface=EGL_NO_SURFACE;}
    if(next)gl_surface=eglCreateWindowSurface(gl_display,gl_config,next,NULL);
}

typedef struct { EGLContext context; EGLSurface scratch; uintptr_t target; uint64_t generation; } GoUIGLContext;

__attribute__((visibility("default"))) uintptr_t goui_host_gles_create(void) {
    if(gl_display==EGL_NO_DISPLAY)return 0;
    GoUIGLContext *c=calloc(1,sizeof(*c));if(!c)return 0;
    const EGLint attrs[]={EGL_CONTEXT_CLIENT_VERSION,3,EGL_NONE};
    const EGLint pixel[]={EGL_WIDTH,1,EGL_HEIGHT,1,EGL_NONE};
    c->context=eglCreateContext(gl_display,gl_config,EGL_NO_CONTEXT,attrs);
    c->scratch=eglCreatePbufferSurface(gl_display,gl_config,pixel);
    if(c->context==EGL_NO_CONTEXT || c->scratch==EGL_NO_SURFACE){
        if(c->context!=EGL_NO_CONTEXT)eglDestroyContext(gl_display,c->context);
        if(c->scratch!=EGL_NO_SURFACE)eglDestroySurface(gl_display,c->scratch);
        free(c);return 0;
    }
    return (uintptr_t)c;
}

__attribute__((visibility("default"))) int32_t goui_host_gles_bind(uintptr_t object,uintptr_t target,uint64_t token) {
    GoUIGLContext *c=(GoUIGLContext*)object;
    EGLSurface surface=target?gl_surface:c->scratch;
    if(target && (target!=(uintptr_t)window || token!=generation || surface==EGL_NO_SURFACE))return -1;
    if(!eglMakeCurrent(gl_display,surface,surface,c->context))return (int32_t)eglGetError();
    c->target=target;c->generation=token;return 0;
}
__attribute__((visibility("default"))) int32_t goui_host_gles_clear(uintptr_t object) {
    (void)object;glFlush();
    return eglMakeCurrent(gl_display,EGL_NO_SURFACE,EGL_NO_SURFACE,EGL_NO_CONTEXT)?0:(int32_t)eglGetError();
}
__attribute__((visibility("default"))) int32_t goui_host_gles_swap(uintptr_t object) {
    GoUIGLContext *c=(GoUIGLContext*)object;
    if(!c->target || c->target!=(uintptr_t)window || c->generation!=generation)return -1;
    return eglSwapBuffers(gl_display,gl_surface)?0:(int32_t)eglGetError();
}
__attribute__((visibility("default"))) int32_t goui_host_gles_interval(uintptr_t object,int32_t interval) {
    (void)object;return eglSwapInterval(gl_display,interval)?0:(int32_t)eglGetError();
}
__attribute__((visibility("default"))) uintptr_t goui_host_gles_proc(const char *name) {
    return (uintptr_t)eglGetProcAddress(name);
}
__attribute__((visibility("default"))) void goui_host_gles_destroy(uintptr_t object) {
    GoUIGLContext *c=(GoUIGLContext*)object;
    eglDestroySurface(gl_display,c->scratch);eglDestroyContext(gl_display,c->context);free(c);
}
