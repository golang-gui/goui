#!/usr/bin/env python3
"""维护者能力验证工具；配置和产物放仓库外，不是公开移动打包命令。"""
import ctypes as C
import ctypes.util
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import zipfile


def shaping_reference(font_path):
    # 独立参考在维护者机器运行，不加载 Android 系统私有库。
    hb = C.CDLL(ctypes.util.find_library("harfbuzz"))
    ptr, uint = C.c_void_p, C.c_uint

    def api(name, result, *args):
        f = getattr(hb, name)
        f.restype, f.argtypes = result, list(args)
        return f

    class Info(C.Structure):
        _fields_ = [(name, uint) for name in ("glyph", "mask", "cluster", "var1", "var2")]

    data = font_path.read_bytes()
    storage = C.create_string_buffer(data)
    blob = api("hb_blob_create", ptr, ptr, uint, C.c_int, ptr, ptr)(storage, len(data), 1, None, None)
    face = api("hb_face_create", ptr, ptr, uint)(blob, 0)
    font = api("hb_font_create", ptr, ptr)(face)
    api("hb_ot_font_set_funcs", None, ptr)(font)
    cases = []
    try:
        for text in ["ffi", "office", "a\u0301", "\u0644\u0627", "\u0633\u0644\u0627\u0645", "\u05d0\u05d1\u05d2", "a\u200bB", "\u200b", "a\u200b\u200bB"]:
            b = api("hb_buffer_create", ptr)()
            try:
                encoded = text.encode("utf-8")
                api("hb_buffer_add_utf8", None, ptr, C.c_char_p, C.c_int, uint, C.c_int)(b, encoded, len(encoded), 0, len(encoded))
                api("hb_buffer_guess_segment_properties", None, ptr)(b)
                api("hb_shape", None, ptr, ptr, ptr, uint)(font, b, None, 0)
                count = uint()
                infos = api("hb_buffer_get_glyph_infos", C.POINTER(Info), ptr, C.POINTER(uint))(b, C.byref(count))
                glyphs = [{"glyph": infos[i].glyph, "cluster": infos[i].cluster} for i in range(count.value)]
                if any(g["glyph"] == 0 for g in glyphs):
                    raise RuntimeError("参考字体缺少用例字形: " + text)
                cases.append({"text": text, "rtl": api("hb_buffer_get_direction", C.c_int, ptr)(b) == 5,
                              "glyphs": glyphs, "boundaries": sorted({g["cluster"] for g in glyphs} | {len(encoded)})})
            finally:
                api("hb_buffer_destroy", None, ptr)(b)
        if len(cases[0]["glyphs"]) != 1:
            raise RuntimeError("参考字体必须包含默认 ffi 连字，如 DejaVuSans.ttf")
        presentation = uint()
        if not api("hb_font_get_nominal_glyph", C.c_int, ptr, uint, C.POINTER(uint))(font, 0xfb03, C.byref(presentation)) or presentation.value != cases[0]["glyphs"][0]["glyph"]:
            raise RuntimeError("参考字体的 U+FB03 必须映射到同一 ffi glyph")
        return {"engine": "HarfBuzz", "version": api("hb_version_string", C.c_char_p)().decode(),
                "fontSha256": hashlib.sha256(data).hexdigest(), "fontFile": font_path.name,
                "ffiPresentationGlyph": presentation.value, "cases": cases}
    finally:
        api("hb_font_destroy", None, ptr)(font)
        api("hb_face_destroy", None, ptr)(face)
        api("hb_blob_destroy", None, ptr)(blob)


def main():
    config = json.loads(Path(sys.argv[1]).read_text())
    root = Path(config["output"]).resolve()
    root.mkdir(parents=True, exist_ok=True)
    source = Path(__file__).resolve().parent
    assets = root / "assets"
    assets.mkdir(exist_ok=True)
    font = Path(config["font"])
    (assets / "shape.json").write_text(json.dumps(shaping_reference(font), ensure_ascii=False, indent=2))
    shutil.copyfile(font, assets / "shape.ttf")
    commands = []

    def run(args, label, env=None):
        with (root / (label + ".log")).open("w") as log:
            p = subprocess.run([str(a) for a in args], stdout=log, stderr=subprocess.STDOUT, env=env)
        commands.append({"step": label, "exit": p.returncode})
        if p.returncode:
            raise RuntimeError(label + "\n" + (root / (label + ".log")).read_text()[-6000:])
        print(label + " passed", flush=True)

    classes, dex = root / "classes", root / "dex"
    classes.mkdir(exist_ok=True)
    dex.mkdir(exist_ok=True)
    run([config["java"], "-m", "jdk.compiler/com.sun.tools.javac.Main", "--release", "8", "-classpath", config["androidJar"],
         "-d", classes, *sorted((source / "native").glob("*.java"))], "maintainer-java")
    run([config["java"], "-cp", config["d8"], "com.android.tools.r8.D8", "--lib", config["androidJar"],
         "--min-api", "29", "--output", dex, *sorted(classes.rglob("*.class"))], "maintainer-dex")
    run([config["cc"], "-O2", "-std=gnu11", "-c", source / "native/android.c", "-o", root / "probe.o"], "maintainer-native")
    repo = source.parents[3]
    workspace = root / "go.work"
    workspace.write_text("go 1.25.8\nuse (\n" + "".join(" " + json.dumps(str(Path(p).resolve())) + "\n" for p in [repo, config["cgo"], config["purego"]]) + ")\n")
    env = os.environ.copy()
    env.update(GOWORK=str(workspace), GOTOOLCHAIN="local", GOFLAGS="", GOCACHE=str(root / "consumer-cache"), PATH=config["consumerPath"])
    run([config["gouiBase"], "build", "-go", config["go"], "-driver", config["driver"], "-linker", config["linker"],
         "-base", config["base"], "-inputs", config["inputs"], "-tools", config["tools"], "-dir", source,
         "-o", root / "libgoui.so", "-object", config["host"], "-object", root / "probe.o", "-library", "android", "-entry", "-x", "."], "consume", env)
    log = (root / "consume.log").read_text()
    if "/cgo -objdir" in log or "# goui base: github.com/golang-gui/goui/platform/mobile/internal/entry" not in log:
        raise RuntimeError("消费构建没有复用固定入口归档")
    if (source / "zz_goui_mobile_main.go").exists():
        raise RuntimeError("构建修改了应用源目录")
    manifest = root / "AndroidManifest.xml"
    manifest.write_text('''<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="org.golanggui.androidtext" android:versionCode="1" android:versionName="0.0.1">
<uses-sdk android:minSdkVersion="29" android:targetSdkVersion="35"/>
<application android:label="GOUI Android text" android:hasCode="true" android:debuggable="true" android:allowBackup="false" android:extractNativeLibs="true" android:theme="@android:style/Theme.Material.Light.NoActionBar">
<activity android:name="android.app.NativeActivity" android:exported="true"><meta-data android:name="android.app.lib_name" android:value="goui"/>
<intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter>
</activity></application></manifest>''')
    unsigned = root / "unsigned.apk"
    run([config["aapt2"], "link", "-I", config["androidJar"], "--manifest", manifest, "-A", assets, "-o", unsigned], "resources")
    with zipfile.ZipFile(unsigned, "a", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.write(root / "libgoui.so", "lib/arm64-v8a/libgoui.so")
        archive.write(dex / "classes.dex", "classes.dex")
    run([config["signer"], "-key", config["key"], "-cert", config["cert"], "-in", unsigned, "-out", root / "probe.apk"], "sign")
    run([config["java"], "-jar", config["apksigner"], "verify", root / "probe.apk"], "verify-signature")
    receipt = {"steps": commands, "consumerCompilerCalls": 0, "go": "1.25.8", "target": "android/arm64",
               "apkSha256": hashlib.sha256((root / "probe.apk").read_bytes()).hexdigest(),
               "hostSha256": hashlib.sha256(Path(config["host"]).read_bytes()).hexdigest()}
    (root / "build-receipt.json").write_text(json.dumps(receipt, indent=2))


if __name__ == "__main__":
    main()
