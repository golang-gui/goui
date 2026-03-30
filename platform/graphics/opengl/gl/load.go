package gl

import (
	"fmt"
	"github.com/golang-gui/goui/core/cgo"
)

var (
	load LoadFunc

	glGetError                 cgo.Symbol
	glBindTexture              cgo.Symbol
	glDeleteTextures           cgo.Symbol
	glGenTextures              cgo.Symbol
	glActiveTexture            cgo.Symbol
	glTexImage2D               cgo.Symbol
	glTexSubImage2D            cgo.Symbol
	glTexParameteri            cgo.Symbol
	glBlendFuncSeparate        cgo.Symbol
	glCreateProgram            cgo.Symbol
	glDeleteProgram            cgo.Symbol
	glGetProgramiv             cgo.Symbol
	glGetProgramInfoLog        cgo.Symbol
	glAttachShader             cgo.Symbol
	glBindAttribLocation       cgo.Symbol
	glLinkProgram              cgo.Symbol
	glUseProgram               cgo.Symbol
	glGetUniformLocation       cgo.Symbol
	glCreateShader             cgo.Symbol
	glDeleteShader             cgo.Symbol
	glGetShaderInfoLog         cgo.Symbol
	glShaderSource             cgo.Symbol
	glCompileShader            cgo.Symbol
	glGetShaderiv              cgo.Symbol
	glGenVertexArrays          cgo.Symbol
	glBindVertexArray          cgo.Symbol
	glGenBuffers               cgo.Symbol
	glBindBuffer               cgo.Symbol
	glDeleteBuffers            cgo.Symbol
	glBufferData               cgo.Symbol
	glEnableVertexAttribArray  cgo.Symbol
	glDisableVertexAttribArray cgo.Symbol
	glVertexAttribPointer      cgo.Symbol
	glDeleteVertexArrays       cgo.Symbol
	glPixelStorei              cgo.Symbol
	glGenerateMipmap           cgo.Symbol
	glUniform1i                cgo.Symbol
	glUniform1fv               cgo.Symbol
	glUniform2fv               cgo.Symbol
	glUniform4fv               cgo.Symbol
	glEnable                   cgo.Symbol
	glDisable                  cgo.Symbol
	glColorMask                cgo.Symbol
	glStencilMask              cgo.Symbol
	glStencilFunc              cgo.Symbol
	glStencilOp                cgo.Symbol
	glStencilOpSeparate        cgo.Symbol
	glDrawArrays               cgo.Symbol
	glCullFace                 cgo.Symbol
	glFrontFace                cgo.Symbol
	glFinish                   cgo.Symbol
	glViewport                 cgo.Symbol
	glClear                    cgo.Symbol
	glClearBufferfv            cgo.Symbol
)

func loadGlFuncs(loadFn LoadFunc) (err error) {
	load = loadFn
	glGetError, err = loadGlFunc("glGetError")
	if err != nil {
		return
	}
	glBindTexture, err = loadGlFunc("glBindTexture")
	if err != nil {
		return
	}
	glDeleteTextures, err = loadGlFunc("glDeleteTextures")
	if err != nil {
		return
	}
	glGenTextures, err = loadGlFunc("glGenTextures")
	if err != nil {
		return
	}
	glActiveTexture, err = loadGlFunc("glActiveTexture")
	if err != nil {
		return
	}
	glTexImage2D, err = loadGlFunc("glTexImage2D")
	if err != nil {
		return
	}
	glTexSubImage2D, err = loadGlFunc("glTexSubImage2D")
	if err != nil {
		return
	}
	glTexParameteri, err = loadGlFunc("glTexParameteri")
	if err != nil {
		return
	}
	glBlendFuncSeparate, err = loadGlFunc("glBlendFuncSeparate")
	if err != nil {
		return
	}
	glCreateProgram, err = loadGlFunc("glCreateProgram")
	if err != nil {
		return
	}
	glDeleteProgram, err = loadGlFunc("glDeleteProgram")
	if err != nil {
		return
	}
	glGetProgramiv, err = loadGlFunc("glGetProgramiv")
	if err != nil {
		return
	}
	glGetProgramInfoLog, err = loadGlFunc("glGetProgramInfoLog")
	if err != nil {
		return
	}
	glAttachShader, err = loadGlFunc("glAttachShader")
	if err != nil {
		return
	}
	glBindAttribLocation, err = loadGlFunc("glBindAttribLocation")
	if err != nil {
		return
	}
	glLinkProgram, err = loadGlFunc("glLinkProgram")
	if err != nil {
		return
	}
	glUseProgram, err = loadGlFunc("glUseProgram")
	if err != nil {
		return
	}
	glGetUniformLocation, err = loadGlFunc("glGetUniformLocation")
	if err != nil {
		return
	}
	glCreateShader, err = loadGlFunc("glCreateShader")
	if err != nil {
		return
	}
	glDeleteShader, err = loadGlFunc("glDeleteShader")
	if err != nil {
		return
	}
	glGetShaderInfoLog, err = loadGlFunc("glGetShaderInfoLog")
	if err != nil {
		return
	}
	glShaderSource, err = loadGlFunc("glShaderSource")
	if err != nil {
		return
	}
	glCompileShader, err = loadGlFunc("glCompileShader")
	if err != nil {
		return
	}
	glGetShaderiv, err = loadGlFunc("glGetShaderiv")
	if err != nil {
		return
	}
	glGenVertexArrays, err = loadGlFunc("glGenVertexArrays")
	if err != nil {
		return
	}
	glBindVertexArray, err = loadGlFunc("glBindVertexArray")
	if err != nil {
		return
	}
	glGenBuffers, err = loadGlFunc("glGenBuffers")
	if err != nil {
		return
	}
	glBindBuffer, err = loadGlFunc("glBindBuffer")
	if err != nil {
		return
	}
	glDeleteBuffers, err = loadGlFunc("glDeleteBuffers")
	if err != nil {
		return
	}
	glBufferData, err = loadGlFunc("glBufferData")
	if err != nil {
		return
	}
	glEnableVertexAttribArray, err = loadGlFunc("glEnableVertexAttribArray")
	if err != nil {
		return
	}
	glDisableVertexAttribArray, err = loadGlFunc("glDisableVertexAttribArray")
	if err != nil {
		return
	}
	glVertexAttribPointer, err = loadGlFunc("glVertexAttribPointer")
	if err != nil {
		return
	}
	glDeleteVertexArrays, err = loadGlFunc("glDeleteVertexArrays")
	if err != nil {
		return
	}
	glPixelStorei, err = loadGlFunc("glPixelStorei")
	if err != nil {
		return
	}
	glGenerateMipmap, err = loadGlFunc("glGenerateMipmap")
	if err != nil {
		return
	}
	glUniform1i, err = loadGlFunc("glUniform1i")
	if err != nil {
		return
	}
	glUniform1fv, err = loadGlFunc("glUniform1fv")
	if err != nil {
		return
	}
	glUniform2fv, err = loadGlFunc("glUniform2fv")
	if err != nil {
		return
	}
	glUniform4fv, err = loadGlFunc("glUniform4fv")
	if err != nil {
		return
	}
	glEnable, err = loadGlFunc("glEnable")
	if err != nil {
		return
	}
	glDisable, err = loadGlFunc("glDisable")
	if err != nil {
		return
	}
	glColorMask, err = loadGlFunc("glColorMask")
	if err != nil {
		return
	}
	glStencilMask, err = loadGlFunc("glStencilMask")
	if err != nil {
		return
	}
	glStencilFunc, err = loadGlFunc("glStencilFunc")
	if err != nil {
		return
	}
	glStencilOp, err = loadGlFunc("glStencilOp")
	if err != nil {
		return
	}
	glStencilOpSeparate, err = loadGlFunc("glStencilOpSeparate")
	if err != nil {
		return
	}
	glDrawArrays, err = loadGlFunc("glDrawArrays")
	if err != nil {
		return
	}
	glCullFace, err = loadGlFunc("glCullFace")
	if err != nil {
		return
	}
	glFrontFace, err = loadGlFunc("glFrontFace")
	if err != nil {
		return
	}
	glFinish, err = loadGlFunc("glFinish")
	if err != nil {
		return
	}
	glViewport, err = loadGlFunc("glViewport")
	if err != nil {
		return
	}
	glClear, err = loadGlFunc("glClear")
	if err != nil {
		return
	}
	glClearBufferfv, err = loadGlFunc("glClearBufferfv")
	if err != nil {
		return
	}

	return nil
}

func loadGlFunc(name string) (symbol cgo.Symbol, err error) {
	fn, err := load(name)
	if err != nil {
		err = fmt.Errorf("gl: load %s err: %v", name, err)
		return
	}
	if fn == 0 {
		err = fmt.Errorf("gl: can not load %s", name)
	}
	return cgo.Symbol(fn), nil
}

func call(fn cgo.Symbol, args ...uintptr) (ret uintptr) {
	ret, _, _ = fn.Call(args...)
	return
}
