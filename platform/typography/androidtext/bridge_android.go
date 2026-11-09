package androidtext

import (
	"fmt"
	"sync"

	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/android/sdk"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
)

const nativeUnits float32 = 64
const componentABI int32 = 3

// layoutPeer binds GOUI's batch adapter, not Android SDK classes. The adapter
// owns only native text/layout state; font registration and raster storage are Go-owned.
type layoutPeer jni.Object

var javaLayout struct {
	sync.Once
	err                                                                       error
	class                                                                     jni.Object
	constructor, close, setSize, setAlignment, setWrap, setTabWidth           jni.Method
	setFont, setColor, setUnderline, setStrike, setPlaceholder, metrics, draw jni.Method
}

func initBridge(e jni.Env) error {
	javaLayout.Do(func() {
		class, err := androidruntime.LoadClass(e, "org.golanggui.typography.LayoutEngine")
		if err != nil {
			javaLayout.err = fmt.Errorf("androidtext: component ABI %d: %w", componentABI, err)
			return
		}
		defer e.DeleteLocalRef(class)
		versionMethod := e.GetStaticMethodID(class, "abiVersion", "()I")
		if err := e.Exception(); err != nil {
			javaLayout.err = fmt.Errorf("androidtext: component ABI %d: %w", componentABI, err)
			return
		}
		if versionMethod == 0 {
			javaLayout.err = fmt.Errorf("androidtext: component ABI %d version method unavailable", componentABI)
			return
		}
		version := e.CallStaticIntMethodA(class, versionMethod)
		if err := e.Exception(); err != nil {
			javaLayout.err = err
			return
		}
		if version != componentABI {
			javaLayout.err = fmt.Errorf("androidtext: component ABI mismatch: need %d, got %d", componentABI, version)
			return
		}
		for _, binding := range []struct {
			name, signature string
			method          *jni.Method
		}{
			{"<init>", "(ILandroid/text/SpannableString;Landroid/text/TextPaint;IIFFF)V", &javaLayout.constructor},
			{"close", "()V", &javaLayout.close}, {"setSize", "(FF)V", &javaLayout.setSize},
			{"setAlignment", "(I)V", &javaLayout.setAlignment}, {"setWrap", "(I)V", &javaLayout.setWrap},
			{"setTabWidth", "(F)V", &javaLayout.setTabWidth},
			{"setFont", "(IILandroid/graphics/Typeface;F)V", &javaLayout.setFont},
			{"setColor", "(III)V", &javaLayout.setColor}, {"setUnderline", "(IIZ)V", &javaLayout.setUnderline},
			{"setStrike", "(IIZ)V", &javaLayout.setStrike}, {"setPlaceholder", "(IIFFF)V", &javaLayout.setPlaceholder},
			{"metrics", "()[Ljava/lang/Object;", &javaLayout.metrics}, {"draw", "(Landroid/graphics/Canvas;)V", &javaLayout.draw},
		} {
			method := e.GetMethodID(class, binding.name, binding.signature)
			if err := e.Exception(); err != nil {
				javaLayout.err = fmt.Errorf("androidtext: component ABI %d %s: %w", componentABI, binding.name, err)
				return
			}
			if method == 0 {
				javaLayout.err = fmt.Errorf("androidtext: method %s unavailable", binding.name)
				return
			}
			*binding.method = method
		}
		javaLayout.class, javaLayout.err = retain(e, class)
	})
	return javaLayout.err
}
func environment() jni.Env {
	e, err := androidruntime.Environment()
	if err != nil {
		panic(err)
	}
	return e
}
func check(e jni.Env) {
	if err := e.Exception(); err != nil {
		panic(err)
	}
}
func frame() jni.Env {
	e := environment()
	if e.PushLocalFrame(32) < 0 {
		check(e)
		panic("androidtext: JNI local frame failed")
	}
	return e
}
func retain(e jni.Env, object jni.Object) (jni.Object, error) {
	if err := e.Exception(); err != nil {
		return 0, err
	}
	if object == 0 {
		return 0, fmt.Errorf("androidtext: null native object")
	}
	ref := e.NewGlobalRef(object)
	if err := e.Exception(); err != nil {
		if ref != 0 {
			e.DeleteGlobalRef(ref)
		}
		return 0, err
	}
	if ref == 0 {
		return 0, fmt.Errorf("androidtext: retain native object failed")
	}
	return ref, nil
}
func nativeTabWidth(value float32) float32 {
	if value == 0 {
		return 20 * nativeUnits
	}
	return value * nativeUnits
}
func (p layoutPeer) invoke(e jni.Env, method jni.Method, args ...jni.Value) error {
	e.CallVoidMethodA(jni.Object(p), method, args...)
	return e.Exception()
}
func (p layoutPeer) close(e jni.Env) error { return p.invoke(e, javaLayout.close) }
func (p layoutPeer) setSize(e jni.Env, w, h float32) error {
	return p.invoke(e, javaLayout.setSize, jni.Float(w*nativeUnits), jni.Float(h*nativeUnits))
}
func (p layoutPeer) setAlignment(e jni.Env, value int32) error {
	return p.invoke(e, javaLayout.setAlignment, jni.Int(value))
}
func (p layoutPeer) setWrap(e jni.Env, value int32) error {
	return p.invoke(e, javaLayout.setWrap, jni.Int(value))
}
func (p layoutPeer) setTabWidth(e jni.Env, value float32) error {
	return p.invoke(e, javaLayout.setTabWidth, jni.Float(nativeTabWidth(value)))
}
func (p layoutPeer) setFont(e jni.Env, start, end int32, face sdk.Typeface, size float32) error {
	return p.invoke(e, javaLayout.setFont, jni.Int(start), jni.Int(end), jni.Ref(jni.Object(face)), jni.Float(size))
}
func (p layoutPeer) setColor(e jni.Env, start, end, color int32) error {
	return p.invoke(e, javaLayout.setColor, jni.Int(start), jni.Int(end), jni.Int(color))
}
func (p layoutPeer) setUnderline(e jni.Env, start, end int32, value bool) error {
	return p.invoke(e, javaLayout.setUnderline, jni.Int(start), jni.Int(end), jni.Bool(value))
}
func (p layoutPeer) setStrike(e jni.Env, start, end int32, value bool) error {
	return p.invoke(e, javaLayout.setStrike, jni.Int(start), jni.Int(end), jni.Bool(value))
}
func (p layoutPeer) setPlaceholder(e jni.Env, start, present int32, w, h, baseline float32) error {
	return p.invoke(e, javaLayout.setPlaceholder, jni.Int(start), jni.Int(present), jni.Float(w*nativeUnits), jni.Float(h*nativeUnits), jni.Float(baseline*nativeUnits))
}
func (p layoutPeer) draw(e jni.Env, canvas sdk.Canvas) error {
	return p.invoke(e, javaLayout.draw, jni.Ref(jni.Object(canvas)))
}

// nativeMetrics is the private batch ABI: UTF-16 indices and native coordinates.
// Typography types and conversion to logical coordinates remain in the Go adapter.
type nativeMetrics struct {
	lineIndices, clusterIndices         []int32
	lineCoordinates, clusterCoordinates []float32
}

func (p layoutPeer) metrics(e jni.Env) (out nativeMetrics, err error) {
	data := e.CallObjectMethodA(jni.Object(p), javaLayout.metrics)
	if err = e.Exception(); err != nil {
		return
	}
	if data == 0 {
		return out, fmt.Errorf("androidtext: null metrics packet")
	}
	if e.GetArrayLength(data) != 4 {
		if err = e.Exception(); err != nil {
			return
		}
		return out, fmt.Errorf("androidtext: invalid metrics packet")
	}
	for i := range 4 {
		array := e.GetObjectArrayElement(data, int32(i))
		if err = e.Exception(); err != nil {
			return
		}
		if array == 0 {
			return out, fmt.Errorf("androidtext: null metrics array %d", i)
		}
		switch i {
		case 0:
			out.lineIndices = e.Ints(array)
		case 1:
			out.lineCoordinates = e.Floats(array)
		case 2:
			out.clusterIndices = e.Ints(array)
		case 3:
			out.clusterCoordinates = e.Floats(array)
		}
		if err = e.Exception(); err != nil {
			return
		}
	}
	return
}
