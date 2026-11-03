#!/usr/bin/python3
# 测试专用 GNU ld 驱动；仅编译基座和验证宿主时允许使用系统 C 编译器。
import os, sys
args = []
for item in sys.argv[1:]:
    if item.endswith(('.c', '.cc', '.m')):
        sys.exit(72)  # Go 链接器的编译器能力探测，明确不支持 C 编译。
    if item in ('-O2', '-g', '-m64', '-Qunused-arguments', '-pthread'):
        continue
    if item.startswith('-Wl,'):
        args.extend(item[4:].split(','))
    elif item == '-rdynamic':
        args.append('--export-dynamic')
    elif item in ('-shared', '-o') or not item.startswith('-') or item.startswith('-l'):
        args.append(item)
    else:
        sys.exit('unsupported link argument: ' + item)
os.execv('/usr/bin/ld', ['/usr/bin/ld', '--eh-frame-hdr', *args, '-lc', '-lpthread', '-ldl'])
