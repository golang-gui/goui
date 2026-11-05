package main

// Go 1.25.8 testing.MainStart 的固定版本适配，仅用于验收工程。
// 不使用 linkname，不伪造 std 内部导入，也不改变官方用例断言。
const runnerSource = `package suite

import (
 "context"
 "crypto/rand"
 "crypto/sha256"
 "crypto/tls"
 "crypto/x509"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "reflect"
 "regexp"
 "runtime"
 "runtime/pprof"
 "sync"
 "testing"
 "time"
)

type Result struct {Name, Status string; Seconds float64}
type Report struct {GoVersion,OS,Arch string; Started time.Time; Seconds float64; Code int; Official int; Results []Result}

func Run(directory string, progress func(string)) Report {
 report:=Report{GoVersion:runtime.Version(),OS:runtime.GOOS,Arch:runtime.GOARCH,Started:time.Now()}
 log,err:=os.Create(filepath.Join(directory,"tests.log"));if err!=nil{panic(err)}
 originalOut,originalErr:=os.Stdout,os.Stderr
 os.Stdout,os.Stderr=log,log
 defer func(){os.Stdout,os.Stderr=originalOut,originalErr;log.Close()}()
 tests:=officialTests();report.Official=len(tests)
 tests=append(tests,testing.InternalTest{"goui-platform/TestPrivateFiles",testPrivateFiles},testing.InternalTest{"goui-platform/TestLocalHTTPS",testLocalHTTPS},testing.InternalTest{"goui-platform/TestLocalDNS",testLocalDNS},testing.InternalTest{"goui-platform/TestRuntimeGC",testRuntimeGC})
 var mu sync.Mutex
 for i:=range tests{
  original:=tests[i].F;name:=tests[i].Name
  tests[i].F=func(t *testing.T){
   started:=time.Now()
   t.Cleanup(func(){status:="pass";if t.Failed(){status="fail"}else if t.Skipped(){status="skip"};mu.Lock();report.Results=append(report.Results,Result{name,status,time.Since(started).Seconds()});mu.Unlock();progress(status+" "+name)})
   original(t)
  }
 }
 m:=testing.MainStart(testDeps{},tests,nil,nil,nil)
 if err=flag.Set("test.v","true");err!=nil{panic(err)}
 if err=flag.Set("test.short","true");err!=nil{panic(err)}
 if err=flag.Set("test.timeout","10m");err!=nil{panic(err)}
 if err=flag.CommandLine.Parse(nil);err!=nil{panic(err)}
 report.Code=m.Run()
 report.Seconds=time.Since(report.Started).Seconds()
 data,err:=json.MarshalIndent(report,"","  ");if err!=nil{panic(err)}
 reportPath:=filepath.Join(directory,"report.json")
 if err=os.WriteFile(reportPath+".tmp",data,0600);err!=nil{panic(err)}
 if err=os.Rename(reportPath+".tmp",reportPath);err!=nil{panic(err)}
 return report
}

func testPrivateFiles(t *testing.T){
 dir:=t.TempDir();path:=filepath.Join(dir,"original.txt");want:=[]byte("Go files: 中文 日本語 한국어 العربية")
 if err:=os.WriteFile(path,want,0600);err!=nil{t.Fatal(err)}
 target:=filepath.Join(dir,"renamed.txt");if err:=os.Rename(path,target);err!=nil{t.Fatal(err)}
 got,err:=os.ReadFile(target);if err!=nil || string(got)!=string(want){t.Fatalf("read back: %q %v",got,err)}
}
func testLocalHTTPS(t *testing.T){
 server:=httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/goui"{http.Error(w,"path",400);return};fmt.Fprint(w,"native Go HTTPS") }));defer server.Close()
 client:=server.Client();client.Timeout=10*time.Second
 // httptest's client trusts only its generated test CA and performs normal TLS verification.
 request,err:=http.NewRequestWithContext(context.Background(),"GET",server.URL+"/goui",nil);if err!=nil{t.Fatal(err)}
 response,err:=client.Do(request);if err!=nil{t.Fatal(err)};defer response.Body.Close()
 body,err:=io.ReadAll(response.Body);if err!=nil || string(body)!="native Go HTTPS" || response.TLS==nil || response.TLS.Version<tls.VersionTLS12 {t.Fatalf("HTTPS: %q %v",body,err)}
 // Default system trust must reject the private httptest certificate.
 if _,err:=server.Certificate().Verify(x509.VerifyOptions{});err==nil{t.Fatal("system roots unexpectedly trust the private test certificate")}
}
func testLocalDNS(t *testing.T){
 ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
 addresses,err:=net.DefaultResolver.LookupIPAddr(ctx,"localhost");if err!=nil{t.Fatal(err)}
 for _,address:=range addresses{if address.IP.IsLoopback(){return}}
 t.Fatalf("localhost did not resolve to a loopback address: %v",addresses)
}
func testRuntimeGC(t *testing.T){
 var data [32]byte;if _,err:=rand.Read(data[:]);err!=nil{t.Fatal(err)}
 expected:=sha256.Sum256(data[:]);copies:=make([][]byte,3000)
 for i:=range copies{copies[i]=append([]byte(nil),data[:]...)}
 for range 4{runtime.GC()};for _,value:=range copies{if sha256.Sum256(value)!=expected{t.Fatal("GC changed live memory")}}
}

// corpusEntry is the exact anonymous type used by testing in Go 1.25.8.
// Unsupported fuzz/profile requests return errors; this runner supplies only tests.
type corpusEntry = struct {Parent string;Path string;Data []byte;Values []any;Generation int;IsSeed bool}
type testDeps struct{}
func (testDeps) ImportPath()string{return "goui-mobile-standard-tests"}
func (testDeps) MatchString(p,s string)(bool,error){return regexp.MatchString(p,s)}
func (testDeps) SetPanicOnExit0(bool){}
func (testDeps) StartCPUProfile(w io.Writer)error{return pprof.StartCPUProfile(w)}
func (testDeps) StopCPUProfile(){pprof.StopCPUProfile()}
func (testDeps) StartTestLog(io.Writer){}
func (testDeps) StopTestLog()error{return nil}
func (testDeps) WriteProfileTo(name string,w io.Writer,debug int)error{p:=pprof.Lookup(name);if p==nil{return errors.New("profile unavailable")};return p.WriteTo(w,debug)}
func (testDeps) CoordinateFuzzing(time.Duration,int64,time.Duration,int64,int,[]corpusEntry,[]reflect.Type,string,string)error{return errors.New("fuzzing is not supported in the mobile test app")}
func (testDeps) RunFuzzWorker(func(corpusEntry)error)error{return errors.New("fuzz workers are not supported")}
func (testDeps) ReadCorpus(string,[]reflect.Type)([]corpusEntry,error){return nil,errors.New("fuzz corpus is not supported")}
func (testDeps) CheckCorpus([]any,[]reflect.Type)error{return errors.New("fuzz corpus is not supported")}
func (testDeps) ResetCoverage(){}
func (testDeps) SnapshotCoverage(){}
func (testDeps) InitRuntimeCoverage()(string,func(string,string)(string,error),func()float64){return "",nil,nil}
`
