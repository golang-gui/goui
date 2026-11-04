// 数据目录只供独立测试，不改变正式平台宿主的 UIKit 生命周期。
#import <Foundation/Foundation.h>
#include <pthread.h>
#include <string.h>
static char *path;
static pthread_once_t once=PTHREAD_ONCE_INIT;
static void Initialize(void){@autoreleasepool{path=strdup([[NSHomeDirectory() stringByAppendingPathComponent:@"Documents"] fileSystemRepresentation]);}}
__attribute__((visibility("default"))) const char *probe_data_dir(void){pthread_once(&once,Initialize);return path;}
