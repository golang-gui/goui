package gl

import "fmt"

var (
	load LoadFunc
	call CallFunc

	glGetError                 uintptr
	glBindTexture              uintptr
	glDeleteTextures           uintptr
	glGenTextures              uintptr
	glActiveTexture            uintptr
	glTexImage2D               uintptr
	glTexSubImage2D            uintptr
	glTexParameteri            uintptr
	glBlendFuncSeparate        uintptr
	glCreateProgram            uintptr
	glDeleteProgram            uintptr
	glGetProgramiv             uintptr
	glGetProgramInfoLog        uintptr
	glAttachShader             uintptr
	glBindAttribLocation       uintptr
	glLinkProgram              uintptr
	glUseProgram               uintptr
	glGetUniformLocation       uintptr
	glCreateShader             uintptr
	glDeleteShader             uintptr
	glGetShaderInfoLog         uintptr
	glShaderSource             uintptr
	glCompileShader            uintptr
	glGetShaderiv              uintptr
	glGenVertexArrays          uintptr
	glBindVertexArray          uintptr
	glGenBuffers               uintptr
	glBindBuffer               uintptr
	glDeleteBuffers            uintptr
	glBufferData               uintptr
	glEnableVertexAttribArray  uintptr
	glDisableVertexAttribArray uintptr
	glVertexAttribPointer      uintptr
	glDeleteVertexArrays       uintptr
	glPixelStorei              uintptr
	glGenerateMipmap           uintptr
	glUniform1i                uintptr
	glUniform2fv               uintptr
	glUniform4fv               uintptr
	glEnable                   uintptr
	glDisable                  uintptr
	glColorMask                uintptr
	glStencilMask              uintptr
	glStencilFunc              uintptr
	glStencilOp                uintptr
	glStencilOpSeparate        uintptr
	glDrawArrays               uintptr
	glCullFace                 uintptr
	glFrontFace                uintptr
	glFinish                   uintptr
	glViewport                 uintptr
	glClear                    uintptr
)

func loadGlFuncs(loadFn LoadFunc, callFn CallFunc) (err error) {
	load, call = loadFn, callFn
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

	return nil
}

func loadGlFunc(name string) (fn uintptr, err error) {
	fn, err = load(name)
	if err != nil {
		err = fmt.Errorf("gl: load %s err: %v", name, err)
		return
	}
	if fn == 0 {
		err = fmt.Errorf("gl: can not load %s", name)
	}
	return
}
