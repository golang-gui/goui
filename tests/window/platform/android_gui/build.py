#!/usr/bin/env python3
"""维护者 APK 验证工具：原生/Dex 制作和无编译器消费明确分开；配置和产物置于仓库外。"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import zipfile


def main():
    config = json.loads(Path(sys.argv[1]).read_text())
    root = Path(config["output"]).resolve()
    root.mkdir(parents=True, exist_ok=True)
    source = Path(__file__).resolve().parent
    repo = source.parents[3]
    steps = []

    def run(args, label, env=None):
        log = root / (label + ".log")
        with log.open("w") as output:
            result = subprocess.run([str(a) for a in args], stdout=output, stderr=subprocess.STDOUT, env=env)
        steps.append({"step": label, "exit": result.returncode})
        if result.returncode:
            raise RuntimeError(label + "\n" + log.read_text()[-6000:])
        print(label + " passed", flush=True)

    classes, dex = root / "classes", root / "dex"
    classes.mkdir(exist_ok=True)
    dex.mkdir(exist_ok=True)
    run([config["java"], "-m", "jdk.compiler/com.sun.tools.javac.Main", "--release", "8", "-classpath", config["androidJar"],
         "-d", classes, *sorted((repo / "platform/typography/androidtext/java").rglob("*.java"))], "maintainer-java")
    run([config["java"], "-cp", config["d8"], "com.android.tools.r8.D8", "--lib", config["androidJar"],
         "--min-api", "29", "--output", dex, *sorted(classes.rglob("*.class"))], "maintainer-dex")
    run([config["cc"], "-O2", "-std=gnu11", "-c", source / "native/android.c", "-o", root / "probe.o"], "maintainer-probe")
    run([config["cc"], "-O2", "-std=gnu11", "-DANativeActivity_onCreate=goui_android_create", "-c",
         repo / "platform/mobile/native/android.c", "-o", root / "host.o"], "maintainer-host")
    workspace = root / "go.work"
    workspace.write_text("go 1.25.8\nuse (\n" + "".join(" " + json.dumps(str(Path(p).resolve())) + "\n"
                         for p in [repo, config["cgo"], config["purego"]]) + ")\n")
    env = os.environ.copy()
    env.update(GOWORK=str(workspace), GOTOOLCHAIN="local", GOFLAGS="", GOCACHE=str(root / "consumer-cache"), PATH=config["consumerPath"])
    run([config["gouiBase"], "build", "-go", config["go"], "-driver", config["driver"], "-linker", config["linker"],
         "-base", config["base"], "-inputs", config["inputs"], "-tools", config["tools"], "-dir", source,
         "-o", root / "libgoui.so", "-object", root / "host.o", "-object", root / "probe.o", "-library", "android", "-entry", "-x", "."], "consume", env)
    log = (root / "consume.log").read_text()
    if "/cgo -objdir" in log or "# goui base: github.com/golang-gui/goui/platform/mobile/internal/entry" not in log:
        raise RuntimeError("消费构建没有复用固定入口归档")
    if (source / "zz_goui_mobile_main.go").exists():
        raise RuntimeError("构建修改了应用源目录")
    manifest = root / "AndroidManifest.xml"
    manifest.write_text('''<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="org.golanggui.androidgui" android:versionCode="1" android:versionName="0.0.1">
<uses-sdk android:minSdkVersion="29" android:targetSdkVersion="35"/>
<application android:label="GOUI Android GUI" android:hasCode="true" android:debuggable="true" android:allowBackup="false" android:extractNativeLibs="true" android:theme="@android:style/Theme.Material.Light.NoActionBar">
<activity android:name="android.app.NativeActivity" android:exported="true"><meta-data android:name="android.app.lib_name" android:value="goui"/>
<intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter>
</activity></application></manifest>''')
    unsigned = root / "unsigned.apk"
    run([config["aapt2"], "link", "-I", config["androidJar"], "--manifest", manifest, "-o", unsigned], "resources")
    with zipfile.ZipFile(unsigned, "a", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.write(root / "libgoui.so", "lib/arm64-v8a/libgoui.so")
        archive.write(dex / "classes.dex", "classes.dex")
    run([config["signer"], "-key", config["key"], "-cert", config["cert"], "-in", unsigned, "-out", root / "probe.apk"], "sign")
    run([config["java"], "-jar", config["apksigner"], "verify", root / "probe.apk"], "verify-signature")
    receipt = {"steps": steps, "consumerCompilerCalls": 0, "go": "1.25.8", "target": "android/arm64",
               "apkSha256": hashlib.sha256((root / "probe.apk").read_bytes()).hexdigest(),
               "hostSha256": hashlib.sha256((root / "host.o").read_bytes()).hexdigest()}
    (root / "build-receipt.json").write_text(json.dumps(receipt, indent=2))


if __name__ == "__main__":
    main()
