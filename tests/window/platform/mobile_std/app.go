package main

// 生成后的应用仅使用公开 platform API；没有 probe 符号、C 宿主或 GUI/UI 依赖。
const applicationSource = `package main

import (
 "encoding/json"
 "fmt"
 "image/color"
 "os"
 "path/filepath"
 "runtime"
 "strings"
 "sync"
 "time"

 "example.org/gouistd/suite"
 "github.com/golang-gui/goui/core/geometry"
 "github.com/golang-gui/goui/platform"
 "github.com/golang-gui/goui/platform/common"
 "github.com/golang-gui/goui/platform/events"
 "github.com/golang-gui/goui/platform/graphics"
 "github.com/golang-gui/goui/platform/typography"
)

func must(err error){if err!=nil{panic(err)}}
func main(){
 must(os.Setenv("GOUI_PLAT_PAINTER","GOUI_SELECTED_PAINTER"))
 p,err:=platform.NewPlatform(platform.DefaultName(),"org.golanggui.StandardTests");must(err);defer p.Destroy()
 loop,err:=p.NewEventLoop();must(err);defer loop.Destroy()
 text,err:=p.NewTypography();must(err);defer text.Destroy()
 directory:=filepath.Join(os.TempDir(),"goui-standard-tests")
 if runtime.GOOS=="ios" {if home,err:=os.UserHomeDir();err==nil && home!="/"{directory=filepath.Join(home,"Documents","GOUIStandardTests")}}
 must(os.MkdirAll(directory,0700))
 var mu sync.Mutex
 var lines []string
 status:="Running official Go 1.25.8 tests"
 nonce:=time.Now().UnixNano()
 var window common.Window
 var painter graphics.Painter
 var layout typography.TextLayout
 var size events.SizeEvent
 visible:=false
 frames:=0
 update:=func(){
  if layout!=nil{layout.Destroy()}
  mu.Lock();body:=strings.Join(lines,"\n");current:=status;mu.Unlock()
  content:=fmt.Sprintf("GOUI / %s / GOUI_SELECTED_PAINTER\n%s\nnonce %d | frames %d\n\n%s\n\nReports: %s",runtime.GOOS,current,nonce,frames,body,directory)
  layout,err=text.NewTextLayout(content,typography.TextFormat{Font:typography.FontInfo{Size:14},TextColor:color.RGBA{R:24,G:40,B:56,A:255},WrapMode:typography.WrapWordChar},max(100,size.Width-32),max(200,size.Height-96));must(err)
 }
 window,err=p.NewWindow(geometry.Size{Width:390,Height:780},func(event events.Event){
  switch e:=event.(type){
  case events.VisibilityEvent:visible=e.Visible
  case events.SizeEvent:size=e;update()
  case events.PaintEvent:
   if size.Width<=0 || size.Height<=0{return}
   if painter==nil{painter,err=p.NewPainter(window);must(err)}
   if layout==nil{update()}
   painter.Begin(size.PixelWidth,size.PixelHeight,size.PixelWidth/size.Width)
   // Conservative example padding; this is not a SafeArea API implementation.
   painter.Clear(graphics.RGB(242,247,252));painter.DrawTextLayout(graphics.Point{X:16,Y:80},layout);painter.End();frames++
   data,_:=json.Marshal(map[string]any{"nonce":nonce,"frames":frames,"width":size.Width,"height":size.Height,"scale":size.PixelWidth/size.Width})
   path:=filepath.Join(directory,"window.json");must(os.WriteFile(path+".tmp",data,0600));must(os.Rename(path+".tmp",path))
  }
 },common.WindowOptions{});must(err)
 defer window.Destroy()
 defer func(){if painter!=nil{painter.Destroy()};if layout!=nil{layout.Destroy()}}()
 must(window.Show())
 done:=make(chan struct{});defer close(done)
 go func(){ticker:=time.NewTicker(time.Second);defer ticker.Stop();for{select{case <-done:return;case <-ticker.C:loop.Post(func(){if visible && window.SurfaceState().Ready{update();must(window.RequestPaint())}})}}}()
 go func(){
  report:=suite.Run(directory,func(line string){mu.Lock();lines=append(lines,line);if len(lines)>12{lines=lines[len(lines)-12:]};mu.Unlock()})
  passed,failed,skipped:=0,0,0
  for _,r:=range report.Results{switch r.Status{case "pass":passed++;case "fail":failed++;case "skip":skipped++}}
  mu.Lock();status=fmt.Sprintf("PASS %d | FAIL %d | SKIP %d\nOfficial %d; elapsed %.2fs; code %d",passed,failed,skipped,report.Official,report.Seconds,report.Code)
  if failed>0{lines=nil;for _,result:=range report.Results{if result.Status=="fail"{lines=append(lines,"FAIL "+result.Name)}}}
  mu.Unlock()
  loop.Post(func(){update();must(window.RequestPaint())})
 }()
 loop.Run()
}
`
