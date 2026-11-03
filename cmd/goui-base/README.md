# Mobile base maintainer tool

This command prepares the native-required Go package archives used by the mobile
build prototype. It has no UI, host-application, or lifecycle responsibilities.
The existing `goui build` interface is unchanged.

The initial supported toolchain is **Go 1.25.8**. The command verifies the build
driver source before generating an adapted `cmd/go` through a temporary overlay;
it never modifies the installed GOROOT. Ordinary Go dependencies still compile
from source. A selected native package with no matching archive fails explicitly.

```sh
go build -o /tmp/goui-base ./cmd/goui-base
/tmp/goui-base driver -go /path/to/go1.25.8/bin/go -o /tmp/go-mobile

# Maintainer production: a real target C compiler is required here.
GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=/path/to/ndk-clang \
  /tmp/goui-base export -go /path/to/go1.25.8/bin/go \
  -dir /path/to/probe -o /tmp/base-android \
  -buildmode c-shared -variant device -minimum 29 ./app

/tmp/goui-base verify /tmp/base-android/base.json
```

For iOS, use `GOOS=ios GOARCH=arm64`, `-buildmode c-archive` and separate device
and simulator components. Production uses the appropriate Apple compiler/SDK.
The base directory must not already exist. No SDK, native executable, or signing
key is checked into this repository.

The generated driver selects a local component through `GOUI_BASE=.../base.json`.
It requires `-trimpath`, the matching target/build mode, unmodified selected
native-package sources, module identity and dependency export data. Native-package
compiler customizations, instrumentation and auxiliary generated outputs are not
supported in this slice. Integrity checks do not authenticate downloaded bundles.

Target linker tools, CRT/stubs, platform hosts, resources and signing remain
separate components. This tool does not yet provide their distribution or a
complete Android/iOS application build command.

See [the design and validation record](../../doc/PlanMobileBuild.md#13-正式基座第一阶段冷缓存归档接入2026-11-03).
Run `go test ./internal/mobilebase ./cmd/goui-base` for logic checks and the
Linux amd64 cold-build integration check. The latter requires Go 1.25.8, cc, ld,
and Python3 for the maintainer/test harness, and does not require a desktop.
