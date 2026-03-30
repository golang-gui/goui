package win32

import (
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics/opengl/gl"
	"runtime"
	"testing"
)

var (
	program  gl.Uint
	VBO, VAO gl.Uint
	glCtx    GlContext
)

func initProgram(t *testing.T) {
	vertShader := gl.CreateShader(gl.GL_VERTEX_SHADER)
	gl.ShaderSource(vertShader, []string{
		`
#version 330 core
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec3 aColor;
out vec3 ourColor;
void main(void) {
	gl_Position = vec4(aPos, 1.0);
	ourColor = aColor;
}
`})
	gl.CompileShader(vertShader)
	success := gl.GetShaderiv(vertShader, gl.GL_COMPILE_STATUS)
	if success == 0 {
		t.Fatal("vertShader compile err:", gl.GetShaderInfoLog(vertShader))
	}

	fragShader := gl.CreateShader(gl.GL_FRAGMENT_SHADER)
	gl.ShaderSource(fragShader, []string{`
#version 330 core
in vec3 ourColor;
out vec4 fragColor;
void main(void) {
	fragColor = vec4(ourColor, 1.0);
}
`})
	gl.CompileShader(fragShader)
	if gl.GetShaderiv(fragShader, gl.GL_COMPILE_STATUS) == 0 {
		t.Fatal("fragShader compile err:", gl.GetShaderInfoLog(fragShader))
	}

	program = gl.CreateProgram()
	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)
	success = gl.GetProgramiv(program, gl.GL_LINK_STATUS)
	if success == 0 {
		t.Fatal("program link err:", gl.GetProgramInfoLog(program))
	}

	VAO = gl.GenVertexArrays(1)[0]
	VBO = gl.GenBuffers(1)[0]

	gl.BindVertexArray(VAO)
	gl.BindBuffer(gl.GL_ARRAY_BUFFER, VBO)

	verts := []float32{
		// 位置          颜色
		-0.5, -0.5, 0, 1, 0, 0, // 左下 红色
		0.5, -0.5, 0, 0, 1, 0, // 右下 绿色
		0, 0.5, 0, 0, 0, 1, // 顶部 蓝色
	}
	gl.BufferData(gl.GL_ARRAY_BUFFER, verts, gl.GL_STATIC_DRAW)

	gl.VertexAttribPointer(0, 3, gl.GL_FLOAT, false, 6*4, 0)
	gl.EnableVertexAttribArray(0)

	gl.VertexAttribPointer(1, 3, gl.GL_FLOAT, false, 6*4, 3*4)
	gl.EnableVertexAttribArray(1)

	//gl.BindBuffer(gl.GL_ARRAY_BUFFER, 0)
	//gl.BindVertexArray(0)
}

func render(width, height uint) {
	gl.Viewport(0, 0, gl.Sizei(width), gl.Sizei(height))

	gl.UseProgram(program)

	gl.BindVertexArray(VAO)

	gl.DrawArrays(gl.GL_TRIANGLES, 0, 3)

	glCtx.SwapBuffers()
}

func TestOpenGLWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	plat, err := NewPlatform()
	if err != nil {
		t.Fatal(err)
	}

	quit := false

	eventQueue, err := plat.NewEventQueue()
	if err != nil {
		t.Fatal(err)
	}

	var width, height uint

	onEvent := func(event events.Event) {
		switch ev := event.(type) {
		case *events.CloseEvent:
			t.Log("window close")
			ev.Window.Destroy()
			quit = true
			eventQueue.Post()
		case *events.SizeEvent:
			t.Logf("window size %dx%d", ev.Width, ev.Height)
			width, height = ev.Width, ev.Height
		case *events.PaintEvent:
			t.Logf("window paint %dx%d", width, height)
			render(width, height)
			ev.Accept()
		case *events.ScaleEvent:
			t.Log("window scale", ev.ScaleFactor)
		}
	}

	win, err := plat.NewWindow(onEvent)
	if err != nil {
		t.Fatal(err)
	}

	scale, _ := win.ScaleFactor()
	t.Log("scale:", scale)

	glCtx, err = NewGlContext(win, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = glCtx.SwapInterval(1)
	if err != nil {
		t.Fatal(err)
	}

	err = glCtx.MakeCurrent()
	if err != nil {
		t.Fatal(err)
	}

	initProgram(t)

	win.SetTitle("TestWindow OpenGL")
	win.Show()

	for !quit {
		eventQueue.Wait()
	}
}
