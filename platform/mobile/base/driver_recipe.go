package base

import (
	"bytes"
	_ "embed"
	"fmt"
)

//go:embed recipe.go
var recipeSource []byte

func recipeDriverSource(data []byte, split bool) ([]byte, error) {
	pathMarker := []byte("\tvar trimpath []string\n")
	if bytes.Count(data, pathMarker) != 1 {
		return nil, fmt.Errorf("ambiguous Go cgo source-path hook")
	}
	data = bytes.Replace(data, pathMarker, append(append([]byte(nil), pathMarker...), []byte("\tif os.Getenv(\"GOUI_RECORD_BASE\") != \"\" { trimpath = append(trimpath, p.Dir+\"=>\"+p.ImportPath) }\n")...), 1)
	markers := []string{
		"if len(p.CgoFiles)+len(p.SwigFiles)+len(p.SwigCXXFiles) > 0 {",
	}
	if !split {
		markers = append(markers, "if p.UsesCgo() || p.UsesSwig() {\n\t\tif pcCFLAGS, pcLDFLAGS, err = b.getPkgConfigFlags(a);")
	}
	for _, marker := range markers {
		if bytes.Count(data, []byte(marker)) != 1 {
			return nil, fmt.Errorf("ambiguous Go object recipe native probe hook")
		}
		at := bytes.IndexByte([]byte(marker), '{')
		replacement := "if (" + marker[3:at] + ") && !gouiUsesObjectRecipe(a) " + marker[at:]
		data = bytes.Replace(data, []byte(marker), []byte(replacement), 1)
	}
	before := []byte("\th := cache.NewHash(\"build \" + p.ImportPath)\n")
	if bytes.Count(data, before) != 1 {
		return nil, fmt.Errorf("ambiguous Go object recipe cache hook")
	}
	data = bytes.Replace(data, before, append(append([]byte(nil), before...), []byte("\tgouiRecipeKey(h, a)\n")...), 1)
	before = []byte("\t\toutGo, outObj, err := b.cgo(a, base.Tool(\"cgo\"), objdir, pcCFLAGS, pcLDFLAGS, mkAbsFiles(p.Dir, cgofiles), gccfiles, cxxfiles, p.MFiles, p.FFiles)\n")
	call := `b.cgo(a, base.Tool("cgo"), objdir, pcCFLAGS, pcLDFLAGS, mkAbsFiles(p.Dir, cgofiles), gccfiles, cxxfiles, p.MFiles, p.FFiles)`
	if split {
		call = `b.processCgoOutputs(a, runCgoPr, base.Tool("cgo"), objdir)`
		before = []byte("\t\toutGo, outObj, err := " + call + "\n")
	}
	if bytes.Count(data, before) != 1 {
		return nil, fmt.Errorf("ambiguous Go object recipe cgo hook")
	}
	after := []byte(`
		var outGo, outObj []string
		var err error
		if gouiBaseManifest != nil && gouiBaseManifest.Format == 2 {
			outGo, outObj, err = gouiRecipeFiles(a)
		} else {
			outGo, outObj, err = GOUI_CGO_CALL
			if err == nil { err = gouiRecordCgo(a, outGo, outObj) }
		}
`)
	after = bytes.ReplaceAll(after, []byte("GOUI_CGO_CALL"), []byte(call))
	return bytes.Replace(data, before, after, 1), nil
}

func recipeActionSource(data []byte) ([]byte, error) {
	marker := []byte("func (b *Builder) cgoAction(p *load.Package, objdir string, deps []*Action, hasCover bool) *Action {\n")
	if bytes.Count(data, marker) != 1 {
		return nil, fmt.Errorf("ambiguous Go split cgo action hook")
	}
	code := []byte(`
	if os.Getenv("GOUI_BASE") != "" {
		return &Action{Mode:"collect cgo", Package:p, Objdir:objdir, Deps:deps,
			Actor:ActorFunc(func(b *Builder, ctx context.Context, a *Action) error {
				if err := ctx.Err(); err != nil { return err }
				return b.gouiRunRecipe(a)
			})}
	}
`)
	return bytes.Replace(data, marker, append(append([]byte(nil), marker...), code...), 1), nil
}

var splitRecipeHook = []byte(`
func (b *Builder) gouiRunRecipe(a *Action) error {
	for _, dep := range a.Deps {
		if p, ok := dep.Provider.(*checkCacheProvider); ok && p.need &^ needCovMetaFile == 0 { return nil }
	}
	if err := b.Shell(a).Mkdir(a.Objdir); err != nil { return err }
	files, _, err := gouiRecipeFiles(a)
	if err != nil { return err }
	a.Provider = &runCgoProvider{goFiles:files}
	return nil
}
`)

var recipeHook = []byte(`
func gouiUsesObjectRecipe(a *Action) bool {
	return gouiBaseError == nil && gouiBaseManifest != nil && gouiBaseManifest.Format == 2 && a.Package.UsesCgo()
}

func gouiRecordCgo(a *Action, generated, objects []string) error {
	dir := os.Getenv("GOUI_RECORD_BASE")
	if dir == "" { return nil }
	p := a.Package
	hash, err := BaseSourceHash(p.Dir, p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles, p.SysoFiles, p.SwigFiles, p.SwigCXXFiles, p.EmbedFiles)
	if err != nil { return err }
	record := BasePackage{SourceSHA256: hash}
	if p.Module != nil {
		record.ModulePath, record.ModuleVersion, record.ModuleGoVersion = p.Module.Path, p.Module.Version, p.Module.GoVersion
	}
	return writeCgoRecord(dir, p.ImportPath, record, generated, objects, []string{a.Objdir+"_cgo_export.h"})
}

func gouiRecipeFiles(a *Action) ([]string, []string, error) {
	m := gouiBaseManifest
	if m == nil || m.Format != 2 { return nil, nil, fmt.Errorf("missing object recipe") }
	if cfg.BuildX { fmt.Fprintf(os.Stderr, "# goui recipe: %s\n", a.Package.ImportPath) }
	p := m.Packages[a.Package.ImportPath]
	var gofiles, objects []string
	for kind, files := range [][]BaseFile{p.GeneratedGo, p.Objects, p.Auxiliary} {
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(m.root, f.Path))
			if err != nil { return nil, nil, err }
			path := a.Objdir+f.Name
			if err = os.WriteFile(path, data, 0600); err != nil { return nil, nil, err }
			if kind == 0 { gofiles = append(gofiles, path) }
			if kind == 1 { objects = append(objects, path) }
		}
	}
	return gofiles, objects, nil
}

func gouiRecipeKey(h interface{Write([]byte)(int,error)}, a *Action) {
	if dir := os.Getenv("GOUI_RECORD_BASE"); dir != "" && a.Package.UsesCgo() { fmt.Fprintln(h, "goui record cgo v1", dir) }
	if os.Getenv("GOUI_BASE") == "" || !a.Package.UsesCgo() { return }
	gouiBaseOnce.Do(func() { gouiBaseManifest, gouiBaseError = BaseOpen(os.Getenv("GOUI_BASE")) })
	if gouiBaseError == nil && gouiBaseManifest.Format == 2 {
		b, _ := os.ReadFile(os.Getenv("GOUI_BASE"))
		fmt.Fprintln(h, "goui object recipe v1", recipeDigest(b))
	}
}
`)
