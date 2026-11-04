// 测试包装：复用正式宿主，仅增加从 Go 线程调用 Java 能力测试的固定入口。
#include <android/native_activity.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern void goui_android_create(ANativeActivity *, void *, size_t);
static JavaVM *vm;
static jclass probe;
static jobject context;
static char *directory;

typedef struct {
    int32_t width, height, stride, length;
    uint8_t *pixels;
    int32_t thread;
} ProbeBitmap;
_Static_assert(sizeof(ProbeBitmap)==32 && offsetof(ProbeBitmap,pixels)==16,"test bitmap ABI");

__attribute__((visibility("default"))) const char *probe_directory(void) { return directory; }
__attribute__((visibility("default"))) void probe_free(ProbeBitmap *b) { if(b){free(b->pixels);free(b);} }
__attribute__((visibility("default"))) ProbeBitmap *probe_run(void) {
    JNIEnv *env=NULL; int attached=0;
    jint result=(*vm)->GetEnv(vm,(void**)&env,JNI_VERSION_1_6);
    if(result==JNI_EDETACHED){if((*vm)->AttachCurrentThread(vm,&env,NULL)!=JNI_OK)return NULL;attached=1;}
    else if(result!=JNI_OK)return NULL;
    ProbeBitmap *out=NULL;
    if((*env)->PushLocalFrame(env,16)<0)goto done;
    jmethodID run=(*env)->GetStaticMethodID(env,probe,"run","(Landroid/content/Context;Ljava/lang/String;)[B");
    jstring path=(*env)->NewStringUTF(env,directory);
    jbyteArray bytes=run?(jbyteArray)(*env)->CallStaticObjectMethod(env,probe,run,context,path):NULL;
    if((*env)->ExceptionCheck(env)){(*env)->ExceptionDescribe(env);(*env)->ExceptionClear(env);goto pop;}
    if(!bytes || (*env)->GetArrayLength(env,bytes)!=800*1100*4)goto pop;
    out=calloc(1,sizeof(*out));if(!out)goto pop;
    out->width=800;out->height=1100;out->stride=3200;out->length=800*1100*4;out->thread=gettid();
    out->pixels=malloc(out->length);if(!out->pixels){free(out);out=NULL;goto pop;}
    (*env)->GetByteArrayRegion(env,bytes,0,out->length,(jbyte*)out->pixels);
    if((*env)->ExceptionCheck(env)){(*env)->ExceptionDescribe(env);(*env)->ExceptionClear(env);probe_free(out);out=NULL;}
pop: (*env)->PopLocalFrame(env,NULL);
done: if(attached)(*vm)->DetachCurrentThread(vm);return out;
}

__attribute__((visibility("default"))) void ANativeActivity_onCreate(ANativeActivity *a,void *saved,size_t size) {
    // 引用归测试进程所有，保留至进程结束；不保留 Activity 或其 JNIEnv。
    if(!probe){
        JNIEnv *env=a->env;vm=a->vm;directory=strdup(a->internalDataPath);
        jclass activityClass=(*env)->GetObjectClass(env,a->clazz);
        jmethodID getContext=(*env)->GetMethodID(env,activityClass,"getApplicationContext","()Landroid/content/Context;");
        jobject app=(*env)->CallObjectMethod(env,a->clazz,getContext);context=(*env)->NewGlobalRef(env,app);
        jmethodID getLoader=(*env)->GetMethodID(env,activityClass,"getClassLoader","()Ljava/lang/ClassLoader;");
        jobject loader=(*env)->CallObjectMethod(env,a->clazz,getLoader);
        jclass loaderClass=(*env)->GetObjectClass(env,loader);
        jmethodID load=(*env)->GetMethodID(env,loaderClass,"loadClass","(Ljava/lang/String;)Ljava/lang/Class;");
        jstring name=(*env)->NewStringUTF(env,"org.golanggui.androidtext.TextProbe");
        jobject cls=(*env)->CallObjectMethod(env,loader,load,name);
        if((*env)->ExceptionCheck(env) || !cls || !context || !directory){(*env)->ExceptionDescribe(env);abort();}
        probe=(*env)->NewGlobalRef(env,cls);
        (*env)->DeleteLocalRef(env,cls);(*env)->DeleteLocalRef(env,name);(*env)->DeleteLocalRef(env,loaderClass);
        (*env)->DeleteLocalRef(env,loader);(*env)->DeleteLocalRef(env,app);(*env)->DeleteLocalRef(env,activityClass);
    }
    goui_android_create(a,saved,size);
}
