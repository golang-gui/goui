package gl

import (
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/core/cgo"
)

type (
	LoadFunc func(symbol string) (fn uintptr, err error)
	CallFunc func(fn uintptr, args ...uintptr) uintptr
)

func Init(loadFn LoadFunc, callFn CallFunc) (err error) {
	return loadGlFuncs(loadFn, callFn)
}

func GetError() Enum {
	ret := call(glGetError)
	return Enum(ret)
}

func BindTexture(target Enum, texture Uint) {
	call(glBindTexture, uintptr(target), uintptr(texture))
}

func GenTexture() (texture Uint) {
	//glGenTextures(GLsizei n, GLuint *textures)
	call(glGenTextures, 1, uintptr(cgo.Pointer(&texture)))
	return
}

func DeleteTexture(texture Uint) {
	//glDeleteTextures(GLsizei n, const GLuint *textures)
	call(glDeleteTextures, 1, uintptr(cgo.Pointer(&texture)))
}

func GenTextures(n Sizei) (textures []Uint) {
	//glGenTextures(GLsizei n, GLuint *textures)
	textures = make([]Uint, n)
	call(glGenTextures, uintptr(len(textures)), uintptr(cgo.CSlice(textures)))
	runtime.KeepAlive(textures)
	return
}

func DeleteTextures(textures []Uint) {
	//glDeleteTextures(GLsizei n, const GLuint *textures)
	call(glDeleteTextures, uintptr(len(textures)), uintptr(cgo.CSlice(textures)))
}

func ActiveTexture(texture Enum) {
	call(glActiveTexture, uintptr(texture))
}

func TexImage2D(target Enum, level Int, internalFormat Int, width, height Sizei, border Int, format, typ Enum, pixels unsafe.Pointer) {
	call(glTexImage2D, uintptr(target), uintptr(level), uintptr(internalFormat), uintptr(width), uintptr(height), uintptr(border), uintptr(format), uintptr(typ), uintptr(pixels))
}

func TexSubImage2D(target Enum, level, xOffset, yOffset Int, width, height Sizei, format, typ Enum, pixels unsafe.Pointer) {
	call(glTexSubImage2D, uintptr(target), uintptr(level), uintptr(xOffset), uintptr(yOffset), uintptr(width), uintptr(height), uintptr(format), uintptr(typ), uintptr(pixels))
}

func TexParameteri(target Enum, name Enum, param Int) {
	call(glTexParameteri, uintptr(target), uintptr(name), uintptr(param))
}

func BlendFuncSeparate(sFactorRgb, dFactorRgb, sFactorAlpha, dFactorAlpha Enum) {
	call(glBlendFuncSeparate, uintptr(sFactorRgb), uintptr(dFactorRgb), uintptr(sFactorAlpha), uintptr(dFactorAlpha))
}

func CreateProgram() Uint {
	ret := call(glCreateProgram)
	return Uint(ret)
}

func DeleteProgram(program Uint) {
	call(glDeleteProgram, uintptr(program))
}

func GetProgramiv(program Uint, name Enum) (value Int) {
	//glGetProgramiv(GLuint program, GLenum pname, GLint *params)
	call(glGetProgramiv, uintptr(program), uintptr(name), uintptr(unsafe.Pointer(&value)))
	return
}

func GetProgramInfoLog(program Uint) string {
	//glGetProgramInfoLog(GLuint program, GLsizei bufSize, GLsizei *length, GLchar *infoLog)
	length := GetProgramiv(program, GL_INFO_LOG_LENGTH)
	if length != 0 {
		buf := make([]byte, length)
		call(glGetProgramInfoLog, uintptr(program), uintptr(length), uintptr(unsafe.Pointer(&length)), uintptr(unsafe.Pointer(cgo.CSlice(buf))))
		return cgo.GoStringNTemp(cgo.CSlice(buf), int(length))
		runtime.KeepAlive(buf)
	}
	return ""
}

func AttachShader(program, shader Uint) {
	call(glAttachShader, uintptr(program), uintptr(shader))
}

func BindAttribLocation(program Uint, index Uint, name string) {
	//glBindAttribLocation(GLuint program, GLuint index, const GLchar *name)
	cName := cgo.CString(name)
	call(glBindAttribLocation, uintptr(program), uintptr(index), uintptr(cName))
	runtime.KeepAlive(cName)
}

func LinkProgram(program Uint) {
	call(glLinkProgram, uintptr(program))
}

func UseProgram(program Uint) {
	call(glUseProgram, uintptr(program))
}

func GetUniformLocation(program Uint, name string) Int {
	//glGetUniformLocation(GLuint program, const GLchar *name)
	cName := cgo.CString(name)
	ret := call(glGetUniformLocation, uintptr(program), uintptr(cName))
	runtime.KeepAlive(cName)
	return Int(ret)
}

func CreateShader(shaderType Enum) Uint {
	ret := call(glCreateShader, uintptr(shaderType))
	return Uint(ret)
}

func DeleteShader(shader Uint) {
	call(glDeleteShader, uintptr(shader))
}

func GetShaderInfoLog(shader Uint) string {
	//glGetShaderInfoLog)(GLuint shader, GLsizei bufSize, GLsizei *length, GLchar *infoLog)
	length := GetShaderiv(shader, GL_INFO_LOG_LENGTH)
	if length != 0 {
		buf := make([]byte, length)
		call(glGetShaderInfoLog, uintptr(shader), uintptr(length), 0, uintptr(unsafe.Pointer(cgo.CSlice(buf))))
		return cgo.GoStringNTemp(cgo.CSlice(buf), int(length))
		runtime.KeepAlive(buf)
	}
	return ""
}

func ShaderSource(shader Uint, sources []string) {
	//glShaderSource(GLuint shader, GLsizei count, const GLchar *const*string, const GLint *length)
	cSources := make([]unsafe.Pointer, len(sources))
	for i := range sources {
		src := cgo.CString(sources[i])
		defer runtime.KeepAlive(src)
		cSources[i] = src
	}
	call(glShaderSource, uintptr(shader), uintptr(len(sources)), uintptr(cgo.CSlice(cSources)), 0)
	runtime.KeepAlive(cSources)
}

func CompileShader(shader Uint) {
	call(glCompileShader, uintptr(shader))
}

func GetShaderiv(shader Uint, name Enum) (value Int) {
	//glGetShaderiv(GLuint shader, GLenum pname, GLint *params)
	call(glGetShaderiv, uintptr(shader), uintptr(name), uintptr(unsafe.Pointer(&value)))
	return
}

func GenVertexArray() (array Uint) {
	//glGenVertexArrays(GLsizei n, GLuint *arrays)
	call(glGenVertexArrays, 1, uintptr(cgo.Pointer(&array)))
	return
}

func DeleteVertexArray(array Uint) {
	//glDeleteVertexArrays(GLsizei n, const GLuint *arrays)
	call(glDeleteVertexArrays, 1, uintptr(cgo.Pointer(&array)))
}

func GenVertexArrays(n Sizei) (arrays []Uint) {
	//glGenVertexArrays(GLsizei n, GLuint *arrays)
	arrays = make([]Uint, n)
	call(glGenVertexArrays, uintptr(len(arrays)), uintptr(cgo.CSlice(arrays)))
	runtime.KeepAlive(arrays)
	return
}

func DeleteVertexArrays(arrays []Uint) {
	//glDeleteVertexArrays(GLsizei n, const GLuint *arrays)
	call(glDeleteVertexArrays, uintptr(len(arrays)), uintptr(cgo.CSlice(arrays)))
}

func BindVertexArray(array Uint) {
	call(glBindVertexArray, uintptr(array))
}

func EnableVertexAttribArray(index Uint) {
	call(glEnableVertexAttribArray, uintptr(index))
}

func DisableVertexAttribArray(index Uint) {
	call(glDisableVertexAttribArray, uintptr(index))
}

func VertexAttribPointer(index Uint, size Int, typ Enum, normalized bool, stride Sizei, pointer uintptr) {
	call(glVertexAttribPointer, uintptr(index), uintptr(size), uintptr(typ), uintptr(cgo.CBool(normalized)), uintptr(stride), pointer)
}

func GenBuffer() (buffer Uint) {
	//glGenBuffers(GLsizei n, GLuint *buffers)
	call(glGenBuffers, 1, uintptr(cgo.Pointer(&buffer)))
	return
}

func DeleteBuffer(buffer Uint) {
	//glDeleteBuffers(GLsizei n, const GLuint *buffers)
	call(glDeleteBuffers, 1, uintptr(cgo.Pointer(&buffer)))
}

func GenBuffers(n Sizei) (buffers []Uint) {
	//glGenBuffers(GLsizei n, GLuint *buffers)
	buffers = make([]Uint, n)
	call(glGenBuffers, uintptr(len(buffers)), uintptr(cgo.CSlice(buffers)))
	runtime.KeepAlive(buffers)
	return
}

func DeleteBuffers(buffers []Uint) {
	//glDeleteBuffers(GLsizei n, const GLuint *buffers)
	call(glDeleteBuffers, uintptr(len(buffers)), uintptr(cgo.CSlice(buffers)))
	runtime.KeepAlive(buffers)
}

func BindBuffer(target Enum, buffer Uint) {
	call(glBindBuffer, uintptr(target), uintptr(buffer))
}

func BufferData[T any](target Enum, data []T, usage Enum) {
	//void glBufferData(GLenum target, GLsizeiptr size, const void *data, GLenum usage)
	var zero T
	call(glBufferData, uintptr(target), uintptr(len(data))*unsafe.Sizeof(zero), uintptr(cgo.CSlice(data)), uintptr(usage))
	runtime.KeepAlive(data)
}

func PixelStorei(name Enum, param Int) {
	call(glPixelStorei, uintptr(name), uintptr(param))
}

func GenerateMipmap(target Enum) {
	call(glGenerateMipmap, uintptr(target))
}

func Uniform1i(location, v1 Int) {
	call(glUniform1i, uintptr(location), uintptr(v1))
}

func Uniform2fv(location Int, values []Float) {
	//glUniform2fv(GLint location, GLsizei count, const GLfloat *value)
	call(glUniform2fv, uintptr(location), uintptr(len(values)), uintptr(cgo.CSlice(values)))
	runtime.KeepAlive(values)
}

func Uniform4fv(location Int, values []Float) {
	//glUniform4fv(GLint location, GLsizei count, const GLfloat *value)
	call(glUniform4fv, uintptr(location), uintptr(len(values)), uintptr(cgo.CSlice(values)))
	runtime.KeepAlive(values)
}

func Enable(cap Enum) {
	call(glEnable, uintptr(cap))
}

func Disable(cap Enum) {
	call(glDisable, uintptr(cap))
}

func ColorMask(red, green, blue, alpha bool) {
	call(glColorMask, uintptr(cgo.CBool(red)), uintptr(cgo.CBool(green)), uintptr(cgo.CBool(blue)), uintptr(cgo.CBool(alpha)))
}

func StencilMask(mask Uint) {
	call(glStencilMask, uintptr(mask))
}

func StencilFunc(fn Enum, ref Int, mask Uint) {
	call(glStencilFunc, uintptr(fn), uintptr(ref), uintptr(mask))
}

func StencilOp(fail, zFail, zPass Enum) {
	call(glStencilOp, uintptr(fail), uintptr(zFail), uintptr(zPass))
}

func StencilOpSeparate(face, sFail, dpFail, dpPass Enum) {
	call(glStencilOpSeparate, uintptr(face), uintptr(sFail), uintptr(dpFail), uintptr(dpPass))
}

func DrawArrays(mode Enum, first Int, count Sizei) {
	call(glDrawArrays, uintptr(mode), uintptr(first), uintptr(count))
}

func CullFace(mode Enum) {
	call(glCullFace, uintptr(mode))
}

func FrontFace(mode Enum) {
	call(glFrontFace, uintptr(mode))
}

func Finish() {
	call(glFinish)
}

func Viewport(x, y Int, width, height Sizei) {
	call(glViewport, uintptr(x), uintptr(y), uintptr(width), uintptr(height))
}

func Clear(mask Bitfield) {
	call(glClear, uintptr(mask))
}
