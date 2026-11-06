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
Run `go test ./platform/mobile/base ./cmd/goui-base` for logic checks and the
Linux amd64 cold-build integration check. The latter requires Go 1.25.8, cc and ld
for the maintainer/test harness, and does not require a desktop.


## Native link components

`goui-base link-inputs -config inputs-spec.json -dir prepared-target -o target`
and `goui-base link-tools -config tools-spec.json -dir prepared-tools -o host`
seal separate target and host components. Source specs use the corresponding
`NativeInputs`/`NativeTools` JSON fields; list each selected relative file path in
`files`, with either an expected SHA-256 or an empty string to calculate it.
Sources, versions and license/review status are required provenance. Output must
not exist. Host tools are checked by executing their declared version.

Host format 1 uses the `lld` executable declaration. Format 2 uses a `golink`
version string (currently `"0.1.0"`), checked against the library compiled into
the existing tools. Its `files` map may be empty: no golink executable is
downloaded or run. Configure exactly one backend for a component. The Go
backend currently supports Android ARM64; iOS requires the existing LLD component. Native support archives retain exact Go version and ABI matching.
Small version-specific bases remain a supported distribution choice.

The internal `goui-link` command consumes `GOUI_LINK_INPUTS` and
`GOUI_LINK_TOOLS`. It adapts a deliberately small set of external-link arguments,
rejects compilation requests and unknown options, and verifies native outputs
before replacing them. Platform host objects/frameworks remain explicit inputs.

For the pinned Go linker's external-archive invocation it reuses `go tool pack`
through `GOUI_GO`. This archive has no native symbol index and is intended only
for the final `-all_load` link; it is not a general-purpose C static library.
No separate llvm-ar or Python runtime is required for consumption.

See [the second-stage design](../../doc/PlanMobileBuild.md#14-第二阶段独立链接组件和-go-链接驱动2026-11-03)
for component identities, SDK boundaries, supported invocation examples and the
remaining distribution/host-validation limitations.
