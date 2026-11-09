// Android 绑定原生验收工程生成器。环境、构建、安装、预期、复位和退出见 README.md。
// 生成到仓库外；验收包归 platform 命名空间，借用既有 internal 启动环境，
// 不为测试增加生产 API、原生代码或预编译组件。
package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	out := flag.String("o", "", "new output directory outside the repository")
	flag.Parse()
	if err := generate(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate(output string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(repo, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(repo)
		if parent == repo {
			return fmt.Errorf("run from the GOUI repository")
		}
		repo = parent
	}
	if output == "" {
		return fmt.Errorf("-o is required")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(repo, output)
	if err != nil {
		return err
	}
	if relative == "." || filepath.IsLocal(relative) {
		return fmt.Errorf("output must be outside the repository")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		return fmt.Errorf("output already exists or is unavailable: %s", output)
	}
	source, err := format.Source([]byte(probeSource))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		return err
	}
	files := map[string][]byte{
		"main.go":   source,
		"go.mod":    []byte("module github.com/golang-gui/goui/platform/validation/androidbindings\n\ngo 1.24\n\nrequire github.com/golang-gui/goui v0.0.0\nreplace github.com/golang-gui/goui => " + strconv.Quote(repo) + "\n"),
		"goui.toml": []byte("id = \"org.golanggui.AndroidBindings\"\nname = \"GOUI Android Bindings\"\nexecutable = \"android-bindings\"\nmain = \".\"\nversion = \"0.0.1\"\nbuild = 1\nicon = " + strconv.Quote(filepath.Join(repo, "doc/logo/goui-mark.svg")) + "\n[android]\ndebuggable = true\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(output, name), data, 0600); err != nil {
			return err
		}
	}
	fmt.Println(output)
	return nil
}

const probeSource = `package main

import (
 "bytes"
 "github.com/goexlib/cgo"
 "github.com/golang-gui/goui/platform/android/libs/bitmap"
    "encoding/json"
    "fmt"
    "math"
    "os"
    "path/filepath"
    "reflect"
    "runtime"
    "strings"
    "syscall"
    "unsafe"

    "github.com/golang-gui/goui/platform"
    native "github.com/golang-gui/goui/platform/android/libs/android"
    "github.com/golang-gui/goui/platform/android/libs/egl"
    "github.com/golang-gui/goui/platform/android/libs/jni"
    "github.com/golang-gui/goui/platform/android/sdk"
    "github.com/golang-gui/goui/platform/internal/androidruntime"
)

var check = "all"

func must(err error){if err!=nil {panic(err)}}
func expect(pass bool,format string,args ...any){if !pass {panic(fmt.Sprintf(format,args...))}}
func main(){
    p,err:=platform.NewPlatform(platform.DefaultName(),"org.golanggui.AndroidBindings");must(err)
    loop,err:=p.NewEventLoop();must(err)
    loop.Post(func(){
        expect(syscall.Gettid()==syscall.Getpid(),"not Android UI thread")
        expect(unsafe.Sizeof(native.Activity{})==80 && unsafe.Sizeof(native.WindowBuffer{})==48,"NDK ABI")
        expect(check=="all" || check=="jni" || check=="egl" || check=="egl-null-query" || check=="text-sdk","unknown check %q",check)
        if check=="all" || check=="jni" {stage("jni");verifyJNI()}
        if check=="text-sdk" {stage("text-sdk");verifyTextSDK()}
        if check=="all" || check=="egl" {stage("egl");verifyEGL()}
        if check=="all" || check=="egl-null-query" {stage("egl-null-query");verifyNullQuery()}
        data,err:=json.Marshal(map[string]any{"passed":true,"check":check,"thread":syscall.Gettid(),"go":runtime.Version()});must(err)
        path:=filepath.Join(os.TempDir(),"bindings.json");must(os.WriteFile(path,data,0600))
        p.Destroy();loop.Quit()
    })
    loop.Run();loop.Destroy()
}
func stage(name string){must(os.WriteFile(filepath.Join(os.TempDir(),"bindings-stage.txt"),[]byte(name),0600))}
func verifyJNI(){
    e,err:=androidruntime.Environment();must(err)
    expect(e.GetVersion()>=jni.Version16,"JNI version")
    expect(e.PushLocalFrame(32)==jni.OK,"local frame")
    _,loadErr:=androidruntime.LoadClass(e,"org.golanggui.bindings.NoSuchClass")
    expect(loadErr!=nil && strings.Contains(loadErr.Error(),"ClassNotFoundException") && !e.ExceptionCheck(),"SDK class-loader exception: %v",loadErr)
    fileClass,loadErr:=androidruntime.LoadClass(e,"java.io.File");must(loadErr)
    file:=e.NewObjectA(fileClass,e.GetMethodID(fileClass,"<init>","(Ljava/lang/String;)V"),jni.Ref(e.NewString("/goui-bindings/中文")));must(e.Exception())
    path,loadErr:=sdk.File(file).GetAbsolutePath(e);must(loadErr)
    expect(path=="/goui-bindings/中文","SDK File path: %q",path)

    text:="A\x00中文😀";value:=e.NewString(text)
    expect(value!=0 && e.String(value)==text,"UTF-16 roundtrip")
    global:=e.NewGlobalRef(value);expect(global!=0,"global reference")
    e.PopLocalFrame(0)
    expect(e.PushLocalFrame(32)==jni.OK,"next frame")
    defer e.PopLocalFrame(0)
    defer e.DeleteGlobalRef(global)
    runtime.GC();expect(e.String(global)==text,"global lifetime")
    expect(e.PushLocalFrame(4)==jni.OK,"promoted-reference frame")
    promoted:=e.PopLocalFrame(e.NewString(text))
    expect(promoted!=0 && e.String(promoted)==text,"local reference promotion")
    expect(e.String(e.NewString(""))=="","empty string")
    ints:=[]int32{math.MinInt32,-1,0,math.MaxInt32}
    expect(reflect.DeepEqual(e.Ints(e.IntArray(ints)),ints),"int array")
    expect(len(e.Ints(e.IntArray(nil)))==0,"empty array")
    stringsArray:=e.Strings([]string{"",text})
    expect(e.GetArrayLength(stringsArray)==2 && e.String(e.GetObjectArrayElement(stringsArray,1))==text,"string array")
    integer:=e.FindClass("java/lang/Integer");must(e.Exception())
    number:=e.CallStaticIntMethodA(integer,e.GetStaticMethodID(integer,"parseInt","(Ljava/lang/String;)I"),jni.Ref(e.NewString("-2147483648")))
    must(e.Exception())
    expect(number==math.MinInt32,"signed jint return: %d",number)
    floats:=e.FindClass("java/lang/Float")
    obj:=e.CallStaticObjectMethodA(floats,e.GetStaticMethodID(floats,"valueOf","(F)Ljava/lang/Float;"),jni.Float(-1.25))
    must(e.Exception())
    f:=e.CallFloatMethodA(obj,e.GetMethodID(floats,"floatValue","()F"));must(e.Exception())
    expect(f==-1.25,"jfloat return")
    builder:=e.FindClass("java/lang/StringBuilder")
    obj=e.NewObjectA(builder,e.GetMethodID(builder,"<init>","()V"))
    must(e.Exception())
    e.CallVoidMethodA(obj,e.GetMethodID(builder,"setLength","(I)V"),jni.Int(3));must(e.Exception())
    length:=e.CallIntMethodA(obj,e.GetMethodID(builder,"length","()I"));must(e.Exception())
    expect(length==3,"void method")
    missing:=e.GetMethodID(builder,"gouiMissingMethod","()V")
    expect(missing==0 && e.ExceptionCheck(),"missing-method exception")
    err=e.Exception();expect(err!=nil && strings.Contains(err.Error(),"NoSuchMethodError") && !e.ExceptionCheck(),"exception formatting/recovery: %v",err)
    expect(e.String(e.NewString("after exception"))=="after exception","JNI recovery")
    for range 32 {expect(e.PushLocalFrame(4)==jni.OK,"stress frame");s:=e.NewString(text);runtime.GC();expect(e.String(s)==text,"GC/stack lifetime");e.PopLocalFrame(0)}
    must(e.Exception())
}
func verifyEGL(){
    display:=egl.GetDisplay(0);expect(display!=0,"EGL display")
    var major,minor int32
    expect(egl.Initialize(display,&major,&minor)==egl.True && major>=1,"EGL initialize")
    defer egl.Terminate(display)
    stage("egl-null-attributes")
    var defaults [1]egl.Config
    count,ok:=egl.ChooseConfig(display,nil,defaults[:])
    expect(ok==egl.True && count==1,"NULL attributes/config output")
    stage("egl-count")
    count,ok=egl.ChooseConfig(display,[]int32{egl.None},nil)
    expect(ok==egl.True && count>0,"NULL attributes/count query: %d",count)
    stage("egl-empty-attributes")
    count,ok=egl.ChooseConfig(display,make([]int32,0,4),defaults[:])
    expect(ok==egl.True && count>0,"empty attributes/count query")
    stage("egl-es3")
    var configs [1]egl.Config
    count,ok=egl.ChooseConfig(display,[]int32{egl.SurfaceType,egl.PbufferBit,egl.RenderableType,egl.OpenGLES3Bit,egl.None},configs[:])
    expect(ok==egl.True && count==1,"ES3 config")
    config:=configs[0]
    context:=egl.CreateContext(display,config,0,[]int32{egl.ContextClientVersion,3,egl.None});expect(context!=0,"context %#x",egl.GetError())
    defer egl.DestroyContext(display,context)
    shared:=egl.CreateContext(display,config,context,[]int32{egl.ContextClientVersion,3,egl.None});expect(shared!=0,"shared context %#x",egl.GetError())
    expect(egl.DestroyContext(display,shared)==egl.True,"destroy shared context")
    surface:=egl.CreatePbufferSurface(display,config,[]int32{egl.Width,4,egl.Height,4,egl.None});expect(surface!=0,"pbuffer")
    defer egl.DestroySurface(display,surface)
    expect(egl.MakeCurrent(display,surface,surface,context)==egl.True,"make current")
    expect(egl.MakeCurrent(display,0,0,0)==egl.True,"clear current")
    // NULL attributes must reach EGL, even when native defaults reject this config.
    empty:=egl.CreateContext(display,config,0,nil)
    if empty!=0 {expect(egl.DestroyContext(display,empty)==egl.True,"empty-attribute context release")} else {expect(egl.GetError()!=egl.Success,"native error was lost")}
}
func verifyNullQuery(){
    display:=egl.GetDisplay(0);expect(display!=0,"EGL display")
    expect(egl.Initialize(display,nil,nil)==egl.True,"EGL initialize")
    defer egl.Terminate(display)
    count,ok:=egl.ChooseConfig(display,nil,nil)
    expect(ok==egl.True && count>0,"NULL attributes/count query")
}

// verifyTextSDK 只调用公开 SDK；几何使用关系断言，不依赖固定字体金图。
func verifyTextSDK() {
 e,err:=androidruntime.Environment();must(err)
 expect(e.PushLocalFrame(96)==jni.OK,"text local frame");defer e.PopLocalFrame(0)
 must(sdk.InitText(e))
 paint,err:=sdk.NewTextPaint(e,sdk.PaintAntiAliasFlag|sdk.PaintSubpixelTextFlag);must(err)
 face,err:=sdk.CreateTypeface(e,"sans-serif",0);must(err)
 face,err=sdk.CreateTypefaceWithWeight(e,face,400,false);must(err)
 _,err=sdk.Paint(paint).SetTypeface(e,face);must(err)
 must(sdk.Paint(paint).SetTextSize(e,20));must(sdk.Paint(paint).SetColor(e,-16777216))
 text:=e.NewString("abc אבג");must(e.Exception())
 spans,err:=sdk.NewSpannableString(e,text);must(err)
 b,err:=sdk.StaticLayoutBuilderObtain(e,jni.Object(spans),0,e.GetStringLength(text),paint,400);must(err)
 b,err=b.SetIncludePad(e,false);must(err)
 align,err:=sdk.LayoutAlignNormal(e);must(err);b,err=b.SetAlignment(e,align);must(err)
 l,err:=b.Build(e);must(err)
 lines,err:=l.GetLineCount(e);must(err);expect(lines==1,"native line count %d",lines)
 width,err:=l.GetLineWidth(e,0);must(err);expect(width>0 && width<400,"native line width %g",width)
 baseline,err:=l.GetLineBaseline(e,0);must(err);expect(baseline>0,"native baseline")
 rtl,err:=l.IsRtlCharAt(e,0);must(err);expect(!rtl,"Latin direction")
 rtl,err=l.IsRtlCharAt(e,4);must(err);expect(rtl,"Hebrew direction")
 file,err:=sdk.NewFile(e,"/system/fonts/Roboto-Regular.ttf");must(err)
 fb,err:=sdk.NewFontBuilder(e,file);must(err);fb,err=fb.SetTtcIndex(e,0);must(err)
 font,err:=fb.Build(e);must(err)
 style,err:=font.GetStyle(e);must(err)
 weight,err:=style.GetWeight(e);must(err);slant,err:=style.GetSlant(e);must(err)
 expect(weight==400 && slant==sdk.FontSlantUpright,"font style %d/%d",weight,slant)
 familyBuilder,err:=sdk.NewFontFamilyBuilder(e,font);must(err)
 family,err:=familyBuilder.Build(e);must(err)
 fallback,err:=sdk.NewTypefaceCustomFallbackBuilder(e,family);must(err)
 fallback,err=fallback.SetSystemFallback(e,"sans-serif");must(err)
 requested,err:=sdk.NewFontStyle(e,500,sdk.FontSlantItalic);must(err)
 fallback,err=fallback.SetStyle(e,requested);must(err)
 custom,err:=fallback.Build(e);must(err);expect(custom!=0,"custom typeface")
 config,err:=sdk.BitmapARGB8888(e);must(err)
 pixels,err:=sdk.CreateBitmap(e,8,8,config);must(err)
 must(pixels.SetDensity(e,sdk.BitmapDensityNone));must(pixels.EraseColor(e,-2130771968))
 info,status:=bitmap.GetInfo(e,jni.Object(pixels));must(e.Exception())
 expect(status==0 && info.Width==8 && info.Height==8 && info.Format==bitmap.RGBA8888,"bitmap format: %+v",info)
 address,status:=bitmap.LockPixels(e,jni.Object(pixels));must(e.Exception());expect(status==0,"bitmap lock")
 sample:=append([]byte(nil),unsafe.Slice((*byte)(cgo.Pointer(address)),4)...)
 status=bitmap.UnlockPixels(e,jni.Object(pixels));must(e.Exception());expect(status==0,"bitmap unlock")
 expect(bytes.Equal(sample,[]byte{128,0,0,128}),"premultiplied SDK bitmap: %v",sample)
 canvas,err:=sdk.NewCanvas(e);must(err);must(canvas.SetBitmap(e,pixels))
 _,err=canvas.Save(e);must(err)
 clipped,err:=canvas.ClipRect(e,0,0,4,4);must(err);expect(clipped,"nonempty clip")
 clipped,err=canvas.ClipRect(e,8,8,16,16);must(err);expect(!clipped,"empty clip")
 must(canvas.Translate(e,1.25,-2.5));must(canvas.Scale(e,2,2));must(canvas.Restore(e))
 must(canvas.SetBitmap(e,0));must(pixels.Recycle(e))
}

`
