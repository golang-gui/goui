// UIKit host. Mutable UIKit/Scene state is confined to the system main thread.
#import <UIKit/UIKit.h>
#import <QuartzCore/QuartzCore.h>
#include <pthread.h>
#include "host.h"

static _Atomic(uintptr_t) callback;
static _Atomic(int) wanted;
static UIView *view;
static UIWindow *window;
static uint64_t generation;
static int width,height,foreground;
static float scale=1;
static pthread_mutex_t image_lock=PTHREAD_MUTEX_INITIALIZER;
static CGImageRef pending_image;
static uintptr_t pending_handle;
static uint64_t pending_generation;
static int image_scheduled;
#if GOUI_GLES
#include "ios_gles.inc"
#endif

static void Emit(int kind,int value){
    GoUIHostCallback fn=(GoUIHostCallback)atomic_load(&callback);if(!fn)return;
    GoUIHostEvent e={kind,kind==GOUI_TARGET?width:value,height,scale,(uintptr_t)(__bridge void*)view,generation};fn(&e);
}
static void ClearImage(void){
    pthread_mutex_lock(&image_lock);
    if(pending_image)CGImageRelease(pending_image);pending_image=NULL;
    pthread_mutex_unlock(&image_lock);
}
static void Lose(void){if(view)Emit(GOUI_LOST,0);ClearImage();generation++;}
static void Target(void){
    if(!view)return;
    float s=window.screen.scale;
    int w=(int)llround(view.bounds.size.width*s),h=(int)llround(view.bounds.size.height*s);
    if(w<=0 || h<=0)return;
    if(w==width && h==height && s==scale)return;
    Lose();width=w;height=h;scale=s;
    view.contentScaleFactor=s;view.layer.contentsScale=s;
#if GOUI_GLES
    atomic_store(&gl_layer,(uintptr_t)(__bridge void*)view.layer);
#endif
    Emit(GOUI_TARGET,0);
}
static void Visibility(void){
    if(view)view.hidden=!atomic_load(&wanted);
    Emit(GOUI_VISIBLE,foreground && view && !view.hidden);
}
__attribute__((visibility("default"))) int32_t goui_host_bind(uintptr_t fn,uint32_t version){
    if(version!=1)return -2;
    atomic_store(&callback,fn);if(!fn)atomic_store(&wanted,0);
    dispatch_async(dispatch_get_main_queue(),^{
        if(!fn){ClearImage();view.layer.contents=nil;}
        Visibility();if(view && width>0){Lose();Emit(GOUI_TARGET,0);}
    });
    return 0;
}
__attribute__((visibility("default"))) int32_t goui_host_visible(int32_t visible){
    atomic_store(&wanted,visible!=0);dispatch_async(dispatch_get_main_queue(),^{Visibility();});return 0;
}
static void FreePixels(void *info,const void *bytes,size_t length){(void)info;(void)length;free((void*)bytes);}
__attribute__((visibility("default"))) int32_t goui_host_present(uintptr_t handle,uint64_t token,const uint8_t *pixels,int32_t w,int32_t h,int32_t stride){
    if(!handle || w<=0 || h<=0 || stride<w*4)return -1;
    size_t size=(size_t)h*stride;void *copy=malloc(size);if(!copy)return -2;memcpy(copy,pixels,size);
    CGDataProviderRef provider=CGDataProviderCreateWithData(NULL,copy,size,FreePixels);
    if(!provider){free(copy);return -2;}
    CGColorSpaceRef color=CGColorSpaceCreateDeviceRGB();
    CGImageRef image=CGImageCreate(w,h,8,32,stride,color,kCGBitmapByteOrder32Big|kCGImageAlphaNoneSkipLast,provider,NULL,false,kCGRenderingIntentDefault);
    CGColorSpaceRelease(color);CGDataProviderRelease(provider);if(!image)return -2;
    pthread_mutex_lock(&image_lock);
    if(pending_image)CGImageRelease(pending_image);
    pending_image=image;pending_handle=handle;pending_generation=token;
    if(!image_scheduled){
        image_scheduled=1;
        dispatch_async(dispatch_get_main_queue(),^{
            pthread_mutex_lock(&image_lock);
            CGImageRef img=pending_image;uintptr_t target=pending_handle;uint64_t epoch=pending_generation;
            pending_image=NULL;image_scheduled=0;
            pthread_mutex_unlock(&image_lock);
            if(img){
                if(atomic_load(&callback) && epoch==generation && target==(uintptr_t)(__bridge void*)view && view){
                    [CATransaction begin];[CATransaction setDisableActions:YES];view.layer.contents=(__bridge id)img;[CATransaction commit];
                }
                CGImageRelease(img);
            }
        });
    }
    pthread_mutex_unlock(&image_lock);return 0;
}
@interface GoUIContent:UIViewController
@end
@implementation GoUIContent
- (void)loadView {
#if GOUI_GLES
    self.view=[[GoUIGLESView alloc]initWithFrame:CGRectZero];
    CAEAGLLayer *layer=(CAEAGLLayer*)self.view.layer;
    layer.opaque=YES;layer.drawableProperties=@{kEAGLDrawablePropertyRetainedBacking:@NO,kEAGLDrawablePropertyColorFormat:kEAGLColorFormatRGBA8};
#else
    self.view=[[UIView alloc]initWithFrame:CGRectZero];
#endif
    self.view.backgroundColor=UIColor.blackColor;
}
- (void)viewDidLayoutSubviews {[super viewDidLayoutSubviews];Target();}
@end
@interface GoUIScene:UIResponder<UIWindowSceneDelegate>
@end
@implementation GoUIScene
- (void)scene:(UIScene*)scene willConnectToSession:(UISceneSession*)session options:(UISceneConnectionOptions*)options{
    (void)session;(void)options;
    window=[[UIWindow alloc]initWithWindowScene:(UIWindowScene*)scene];
    window.rootViewController=[[GoUIContent alloc]init];view=window.rootViewController.view;
    width=height=0;[window makeKeyAndVisible];Target();Visibility();
}
- (void)sceneWillEnterForeground:(UIScene*)scene{(void)scene;foreground=1;
#if GOUI_GLES
    atomic_store(&gl_allowed,1);width=height=0;Target();
#endif
    Visibility();
}
- (void)sceneDidEnterBackground:(UIScene*)scene{(void)scene;foreground=0;
#if GOUI_GLES
    HostGLSuspend();Lose();atomic_store(&gl_layer,0);
#endif
    Visibility();
}
- (void)sceneDidBecomeActive:(UIScene*)scene{(void)scene;Emit(GOUI_FOCUS,window.isKeyWindow);}
- (void)sceneWillResignActive:(UIScene*)scene{(void)scene;Emit(GOUI_FOCUS,0);}
- (void)sceneDidDisconnect:(UIScene*)scene{(void)scene;
#if GOUI_GLES
    HostGLSuspend();
#endif
    Lose();foreground=0;Visibility();
#if GOUI_GLES
    atomic_store(&gl_layer,0);
#endif
    window.hidden=YES;window.rootViewController=nil;view=nil;window=nil;width=height=0;
}
@end
@interface GoUIDelegate:UIResponder<UIApplicationDelegate>
@end
@implementation GoUIDelegate
- (BOOL)application:(UIApplication*)app didFinishLaunchingWithOptions:(NSDictionary*)options{(void)app;(void)options;return goui_run()==0;}
@end
int main(int argc,char **argv){@autoreleasepool{return UIApplicationMain(argc,argv,nil,NSStringFromClass([GoUIDelegate class]));}}
