# Embedded mobile inputs

These generated Go files contain compressed target data, not a native toolchain.
They are linked into `cmd/goui` only; applications do not embed the build inputs.

The Go-version bundle contains only native-required support archives. The target
bundles contain CRT/compiler support, system link interfaces and the small
Android Dex or iOS entry/protocol objects. No `android.jar`, aapt2, LLD, Java,
host executable, private key or prebuilt application is included.

Regenerate with Go 1.25.8 from verified maintainer profiles:

```sh
GOUI_ANDROID_COMPONENT=/path/components/android-arm64 \
GOUI_IOS_COMPONENT=/path/components/ios-arm64 \
GOUI_IOS_SIMULATOR_COMPONENT=/path/components/ios-arm64-simulator \
go generate ./cmd/goui/internal/mobileassets
```

Production of the input archives still requires the matching native maintainer
toolchains; consumption does not. The generator verifies every consumed profile
digest. It excludes host tools and `android.jar`, preserves exact archive bytes,
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
