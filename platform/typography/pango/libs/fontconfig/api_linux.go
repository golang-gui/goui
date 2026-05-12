package fontconfig

import (
	"github.com/goexlib/cgo"
	"runtime"
	"unsafe"
)

var (
	libfontconfig = cgo.NewLazyLibrary("libfontconfig.so.1")

	fcConfigCreate         = libfontconfig.NewSymbol("FcConfigCreate")
	fcConfigDestroy        = libfontconfig.NewSymbol("FcConfigDestroy")
	fcConfigParseAndLoad   = libfontconfig.NewSymbol("FcConfigParseAndLoad")
	fcConfigAppFontAddFile = libfontconfig.NewSymbol("FcConfigAppFontAddFile")
	fcPatternDestroy       = libfontconfig.NewSymbol("FcPatternDestroy")
	fcPatternGetString     = libfontconfig.NewSymbol("FcPatternGetString")
	fcFreeTypeQuery        = libfontconfig.NewSymbol("FcFreeTypeQuery")
	fcFreeTypeQueryAll     = libfontconfig.NewSymbol("FcFreeTypeQueryAll")
)

// ConfigCreate creates a new empty configuration.
func ConfigCreate() Config {
	// FcConfig* FcConfigCreate()
	ret, _, _ := fcConfigCreate.CallRaw()
	return Config(ret)
}

// Destroy destroys a configuration and any data associated with it.
func (c Config) Destroy() {
	// void FcConfigDestroy(FcConfig* config)
	fcConfigDestroy.CallRaw(uintptr(c))
}

// ParseAndLoad loads the configuration from the given file.
// Pass an empty string for file to load the default system configuration.
// If complain is true, errors are printed to stderr.
func (c Config) ParseAndLoad(file string, complain bool) Bool {
	// FcBool FcConfigParseAndLoad(FcConfig* config, const FcChar8* file, FcBool complain)
	var filePtr uintptr
	if file != "" {
		cFile := cgo.CString(file)
		filePtr = uintptr(cFile)
		defer runtime.KeepAlive(cFile)
	}
	ret, _, _ := fcConfigParseAndLoad.CallRaw(uintptr(c), filePtr, uintptr(cgo.CBool(complain)))
	return Bool(ret)
}

// AppFontAddFile adds an application-specific font file to the configuration.
// Returns true if the font was successfully added.
func (c Config) AppFontAddFile(file string) Bool {
	// FcBool FcConfigAppFontAddFile(FcConfig* config, const FcChar8* file)
	cFile := cgo.CString(file)
	ret, _, _ := fcConfigAppFontAddFile.CallRaw(uintptr(c), uintptr(cFile))
	runtime.KeepAlive(cFile)
	return Bool(ret)
}

// Destroy destroys a pattern, freeing all memory associated with it.
func (p Pattern) Destroy() {
	// void FcPatternDestroy(FcPattern* p)
	fcPatternDestroy.CallRaw(uintptr(p))
}

// GetString returns the string value of the named object at position n in the pattern.
// Returns the string and ResultMatch on success.
func (p Pattern) GetString(object string, n int) (string, Result) {
	// FcResult FcPatternGetString(const FcPattern* p, const char* object, int n, FcChar8** s)
	cObject := cgo.CString(object)
	var strPtr uintptr
	ret, _, _ := fcPatternGetString.CallRaw(uintptr(p), uintptr(cObject), uintptr(n), uintptr(unsafe.Pointer(&strPtr)))
	runtime.KeepAlive(cObject)
	if Result(ret) == ResultMatch && strPtr != 0 {
		return cgo.GoString(cgo.Pointer(strPtr)), ResultMatch
	}
	return "", Result(ret)
}

// FreeTypeQuery queries a font file for its properties.
// file is the path to the font file, id is the face index within the file (0 for the first face).
// Returns a Pattern with the font's properties, or 0 on failure.
// The caller must call Pattern.Destroy() when done.
func FreeTypeQuery(file string, id int) Pattern {
	// FcPattern* FcFreeTypeQuery(const FcChar8* file, unsigned int id, FcBlanks* blanks, int* count)
	cFile := cgo.CString(file)
	var count int32
	ret, _, _ := fcFreeTypeQuery.CallRaw(uintptr(cFile), uintptr(id), 0, uintptr(unsafe.Pointer(&count)))
	runtime.KeepAlive(cFile)
	return Pattern(ret)
}

// FreeTypeQueryAll queries all faces in a font file (useful for TTC collections).
// Returns the number of faces found.
// Each face can be queried individually with FreeTypeQuery(file, i).
func FreeTypeQueryAll(file string) int {
	// unsigned int FcFreeTypeQueryAll(const FcChar8* file, unsigned int id, FcBlanks* blanks, int* count, FcFontSet* set)
	// Pass id=-1 to count all faces without adding to a set
	cFile := cgo.CString(file)
	var count int32
	fcFreeTypeQueryAll.CallRaw(uintptr(cFile), uintptr(0xFFFFFFFF), 0, uintptr(unsafe.Pointer(&count)), 0)
	runtime.KeepAlive(cFile)
	return int(count)
}
