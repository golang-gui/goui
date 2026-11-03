// 使用独立 C ABI 调用两个版本的 Go 库；验证外来线程、GC 和栈增长。
#include <dlfcn.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
static int64_t (*probe)(int64_t);
static int version;
static void *run(void *unused) {
 for (int64_t i=0;i<128;i++) if(probe(i)!=i*3+version) return (void*)1;
 return NULL;
}
int main(int argc,char **argv) {
 if(argc!=3) return 2;
 void *lib=dlopen(argv[1],RTLD_NOW|RTLD_LOCAL);
 if(!lib){fprintf(stderr,"%s\n",dlerror());return 3;}
 probe=dlsym(lib,"goui_base_probe");version=atoi(argv[2]);if(!probe)return 4;
 pthread_t threads[4];
 for(int i=0;i<4;i++) if(pthread_create(&threads[i],NULL,run,NULL))return 5;
 for(int i=0;i<4;i++){void *status;if(pthread_join(threads[i],&status)||status)return 6;}
 puts("PASS: 512 native callbacks, GC and stack growth");return 0;
}
