// Android host: native callbacks and UI operations stay on the Android thread.
// The Go application and software painting run on the separate GOUI thread.
#include <android/native_activity.h>
#include <android/native_window.h>
#include <android/configuration.h>
#include <android/looper.h>
#include <android/input.h>
#include <pthread.h>
#include <sys/eventfd.h>
#include <unistd.h>
#include <string.h>
#include <stdlib.h>
#include "host.h"

static _Atomic(uintptr_t) callback;
static _Atomic(int) wanted;
static pthread_mutex_t command_lock=PTHREAD_MUTEX_INITIALIZER;
static int wake_fd=-1;
// The fields below are only accessed on the Android UI thread.
static ANativeActivity *activity;
static ANativeWindow *window;
static uint64_t generation;
static int width,height,started,shown;
static float density=1;
static AInputQueue *input;

static void Emit(int kind,int value) {
    GoUIHostCallback fn=(GoUIHostCallback)atomic_load(&callback);
    if (!fn) return;
    GoUIHostEvent e={kind,kind==GOUI_TARGET?width:value,height,density,(uintptr_t)window,generation};
    fn(&e);
}
static void Revoke(void) {
    if (window) Emit(GOUI_LOST,0); // Go waits for any native present in progress.
}
static void Target(ANativeWindow *next) {
    int w=next?ANativeWindow_getWidth(next):0,h=next?ANativeWindow_getHeight(next):0;
    if (next==window && w==width && h==height) return;
    Revoke(); window=next; width=w; height=h; generation++;
    if (window && width>0 && height>0) Emit(GOUI_TARGET,0);
}
static void Wake(void) {
    pthread_mutex_lock(&command_lock);
    if(wake_fd>=0){uint64_t one=1; (void)write(wake_fd,&one,sizeof(one));}
    pthread_mutex_unlock(&command_lock);
}
// JNI locals are confined to this call and the system UI thread.
static void Visibility(void) {
    if (!activity) return;
    JNIEnv *env=activity->env;
    jclass ac=(*env)->GetObjectClass(env,activity->clazz);
    jmethodID getWindow=(*env)->GetMethodID(env,ac,"getWindow","()Landroid/view/Window;");
    jobject win=(*env)->CallObjectMethod(env,activity->clazz,getWindow);
    jclass wc=(*env)->GetObjectClass(env,win);
    jmethodID getDecor=(*env)->GetMethodID(env,wc,"getDecorView","()Landroid/view/View;");
    jobject decor=(*env)->CallObjectMethod(env,win,getDecor);
    jclass vc=(*env)->GetObjectClass(env,decor);
    jmethodID set=(*env)->GetMethodID(env,vc,"setVisibility","(I)V");
    jmethodID get=(*env)->GetMethodID(env,vc,"getVisibility","()I");
    (*env)->CallVoidMethod(env,decor,set,atomic_load(&wanted)?0:4);
    shown=(*env)->CallIntMethod(env,decor,get)==0;
    (*env)->DeleteLocalRef(env,vc); (*env)->DeleteLocalRef(env,decor);
    (*env)->DeleteLocalRef(env,wc); (*env)->DeleteLocalRef(env,win); (*env)->DeleteLocalRef(env,ac);
    Emit(GOUI_VISIBLE,started && shown);
}
static int Commands(int fd,int events,void *data) {
    (void)events; (void)data;
    uint64_t count; (void)read(fd,&count,sizeof(count));
    Visibility();
    if (window) { generation++; Emit(GOUI_TARGET,0); }
    return 1;
}
__attribute__((visibility("default"))) int32_t goui_host_bind(uintptr_t fn,uint32_t version) {
    if(version!=1) return -2;
    atomic_store(&callback,fn);
    if(!fn) atomic_store(&wanted,0);
    Wake(); return 0;
}
__attribute__((visibility("default"))) int32_t goui_host_visible(int32_t visible) {
    atomic_store(&wanted,visible!=0); Wake(); return 0;
}
__attribute__((visibility("default"))) int32_t goui_host_present(uintptr_t handle,uint64_t token,const uint8_t *pixels,int32_t w,int32_t h,int32_t stride) {
    (void)token;
    ANativeWindow *target=(ANativeWindow*)handle;
    if (!target || w<=0 || h<=0 || stride<w*4) return -1;
    ANativeWindow_Buffer buffer;
    if (ANativeWindow_lock(target,&buffer,NULL)!=0) return -1;
    if(buffer.format!=WINDOW_FORMAT_RGBA_8888 && buffer.format!=WINDOW_FORMAT_RGBX_8888){ANativeWindow_unlockAndPost(target);return -2;}
    // Native resize can precede its callback. Never read outside either image.
    int rows=h<buffer.height?h:buffer.height,columns=w<buffer.width?w:buffer.width;
    for(int y=0;y<buffer.height;y++){
        uint8_t *dst=(uint8_t*)buffer.bits+(size_t)y*buffer.stride*4;
        memset(dst,0,(size_t)buffer.width*4);
        for(int x=0;x<buffer.width;x++) dst[x*4+3]=255;
        if(y<rows) for(int x=0;x<columns;x++){
            memcpy(dst+x*4,pixels+(size_t)y*stride+x*4,3); dst[x*4+3]=255;
        }
    }
    return ANativeWindow_unlockAndPost(target)==0?0:-1;
}
static void OnStart(ANativeActivity *a){(void)a; started=1; Visibility();}
static void OnStop(ANativeActivity *a){(void)a; started=0; Emit(GOUI_VISIBLE,0);}
static void OnFocus(ANativeActivity *a,int focus){(void)a; Emit(GOUI_FOCUS,focus);}
static void OnWindow(ANativeActivity *a,ANativeWindow *w){(void)a; ANativeWindow_setBuffersGeometry(w,0,0,WINDOW_FORMAT_RGBX_8888); Target(w);}
static void OnResize(ANativeActivity *a,ANativeWindow *w){(void)a; Target(w);}
static void OnRedraw(ANativeActivity *a,ANativeWindow *w){(void)a; Target(w); Emit(GOUI_REDRAW,0);}
static void OnLost(ANativeActivity *a,ANativeWindow *w){(void)a; if(window==w) Target(NULL);}
static void OnConfiguration(ANativeActivity *a){
    AConfiguration *config=AConfiguration_new(); AConfiguration_fromAssetManager(config,a->assetManager);
    int dpi=AConfiguration_getDensity(config); AConfiguration_delete(config);
    float scale=dpi>0 && dpi<0xfffe?dpi/160.f:1;
    if(scale!=density){Revoke(); density=scale; generation++; if(window) Emit(GOUI_TARGET,0);}
}
static void OnDestroy(ANativeActivity *a){
    Target(NULL); Emit(GOUI_VISIBLE,0);
    pthread_mutex_lock(&command_lock);
    int fd=wake_fd; wake_fd=-1;
    if(fd>=0){ALooper_removeFd(ALooper_forThread(),fd); close(fd);}
    pthread_mutex_unlock(&command_lock);
    if(activity==a)activity=NULL;
    if(input){AInputQueue_detachLooper(input);input=NULL;}
}
static int DrainInput(int fd,int events,void *data){
    (void)fd;(void)events;(void)data;AInputEvent *event;
    while(input && AInputQueue_getEvent(input,&event)>=0){
        if(AInputQueue_preDispatchEvent(input,event))continue;
        AInputQueue_finishEvent(input,event,0);
    }
    return 1;
}
static void OnInput(ANativeActivity *a,AInputQueue *q){(void)a;input=q;AInputQueue_attachLooper(q,ALooper_forThread(),ALOOPER_POLL_CALLBACK,DrainInput,NULL);}
static void OnInputLost(ANativeActivity *a,AInputQueue *q){(void)a;AInputQueue_detachLooper(q);if(input==q)input=NULL;}
__attribute__((visibility("default"))) void ANativeActivity_onCreate(ANativeActivity *a,void *saved,size_t size){
    (void)saved;(void)size; activity=a; started=0;
    pthread_mutex_lock(&command_lock);
    wake_fd=eventfd(0,EFD_NONBLOCK|EFD_CLOEXEC);
    if(wake_fd<0)abort();
    ALooper_addFd(ALooper_forThread(),wake_fd,ALOOPER_POLL_CALLBACK,ALOOPER_EVENT_INPUT,Commands,NULL);
    pthread_mutex_unlock(&command_lock);
    ANativeActivityCallbacks *c=a->callbacks;
    c->onStart=OnStart;c->onStop=OnStop;c->onWindowFocusChanged=OnFocus;
    c->onNativeWindowCreated=OnWindow;c->onNativeWindowDestroyed=OnLost;
    c->onNativeWindowResized=OnResize;c->onNativeWindowRedrawNeeded=OnRedraw;
    c->onConfigurationChanged=OnConfiguration;c->onDestroy=OnDestroy;
    c->onInputQueueCreated=OnInput;c->onInputQueueDestroyed=OnInputLost;
    OnConfiguration(a);
    if(goui_run()!=0)abort();
}
