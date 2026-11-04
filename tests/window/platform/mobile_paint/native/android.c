// 测试仅公开沙箱目录与线程事实；生命周期和绘制使用正式 Android 宿主。
#include <android/native_activity.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <pthread.h>
extern void goui_android_create(ANativeActivity *,void *,size_t);
static char *path;
__attribute__((visibility("default"))) const char *probe_data_dir(void){return path;}
__attribute__((visibility("default"))) uintptr_t probe_thread(void){return (uintptr_t)pthread_self();}
__attribute__((visibility("default"))) int probe_is_main(void){return gettid()==getpid();}
__attribute__((visibility("default"))) void ANativeActivity_onCreate(ANativeActivity *a,void *saved,size_t size){
    if(!path){
        path=strdup(a->internalDataPath);
        char file[4096];int n=snprintf(file,sizeof(file),"%s/go.stderr",path);
        if(n>0 && n<(int)sizeof(file)){int fd=open(file,O_CREAT|O_WRONLY|O_TRUNC,0600);if(fd>=0){dup2(fd,STDERR_FILENO);close(fd);}}
    }
    goui_android_create(a,saved,size);
}
