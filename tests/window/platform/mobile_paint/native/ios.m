// 测试仅公开沙箱目录、线程与已加载库检查，不替代正式 UIKit 宿主或文字实现。
#import <Foundation/Foundation.h>
#include <dlfcn.h>
#include <pthread.h>
#include <string.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
static char *path;
static pthread_once_t once=PTHREAD_ONCE_INIT;
static void Initialize(void){@autoreleasepool{
    NSString *directory=[NSHomeDirectory() stringByAppendingPathComponent:@"Documents"];
    [[NSFileManager defaultManager]createDirectoryAtPath:directory withIntermediateDirectories:YES attributes:nil error:nil];
    path=strdup(directory.fileSystemRepresentation);
    int fd=open([[directory stringByAppendingPathComponent:@"go.stderr"]fileSystemRepresentation],O_CREAT|O_WRONLY|O_TRUNC,0600);
    if(fd>=0){dup2(fd,STDERR_FILENO);close(fd);}
}}
__attribute__((visibility("default"))) const char *probe_data_dir(void){pthread_once(&once,Initialize);return path;}
__attribute__((visibility("default"))) uintptr_t probe_thread(void){return (uintptr_t)pthread_self();}
__attribute__((visibility("default"))) int probe_is_main(void){return pthread_main_np();}
__attribute__((visibility("default"))) int probe_desktop_loaded(void){
    int result=0;
    const char *paths[]={"/System/Library/Frameworks/AppKit.framework/AppKit","/System/Library/Frameworks/OpenGL.framework/OpenGL"};
    for(int i=0;i<2;i++){void *h=dlopen(paths[i],RTLD_NOW|RTLD_NOLOAD);if(h){dlclose(h);result|=1<<i;}}
    return result;
}
