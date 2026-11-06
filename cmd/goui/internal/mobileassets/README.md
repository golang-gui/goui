# Embedded mobile inputs

These generated Go files contain compressed target data, not a native toolchain.
They are linked into `cmd/goui` only; applications do not embed the build inputs.

The Go-version bundle contains only a native-support recipe. Shared pools contain
cgo-generated Go source and native objects; ordinary Go is compiled on demand. The target
bundles contain CRT/compiler support, system link interfaces and the small
Android Dex or iOS entry/protocol objects. No `android.jar`, aapt2, LLD, Java,
host executable, private key or prebuilt application is included.

Regenerate one complete Go-version set from verified maintainer profiles:

```sh
GOUI_ANDROID_COMPONENT=/path/components/android-arm64 \
GOUI_IOS_COMPONENT=/path/components/ios-arm64 \
GOUI_IOS_SIMULATOR_COMPONENT=/path/components/ios-arm64-simulator \
go generate ./cmd/goui/internal/mobileassets
```

Production of the native inputs still requires the matching native maintainer
toolchains; consumption does not. The generator verifies every consumed profile
digest. It excludes host tools and `android.jar`, preserves exact native object bytes,
and projects TBD documents/export groups to ARM64 device or simulator. All
matching symbols, reexports and library versions remain; it does not prune
symbols based on one sample application. The simulator SDK's three documented
libSystem host reexports retain their original macOS target tags.

Shared target inputs are stored independently of the Go version. Generated
manifests retain upstream provenance and the original licensing-review status.
The Go runtime is BSD licensed; third-party support code follows its respective
module license. SDK-derived inputs require a redistribution review before public
publication. Embedding or projecting those inputs does not grant redistribution
rights. See `doc/DesignMobileBuild.md` for supported versions and acceptance gaps.

Object-level bases use format 2. Each target has one content-addressed pool of
native objects and generated Go sources, shared across supported Go versions.
Each Go version retains its own recipe and exact source/toolchain checks. Normal
Go source, assembly, export data and package archives are compiled by the user.

To regenerate several versions together, supply a JSON array containing every
verified profile directory (three targets for each Go version):

```sh
GOUI_MOBILE_PROFILES=/absolute/profiles.json go generate ./cmd/goui/internal/mobileassets
```

The generator requires identical target resources across versions and stores
identical object bytes once. It rejects incomplete version sets or complete Go
archives. No compiler or producer directory is embedded in the generated input.

Current embedded recipes cover Go 1.24.13, 1.25.8, 1.26.10 and 1.27.3 for Android
ARM64, iOS ARM64 device and simulator. A different patch release is not assumed
compatible. Adding a version requires verified producer records and build/runtime
acceptance; identical files in its recipes reuse the existing target object pool.
