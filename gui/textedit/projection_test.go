package textedit

import (
	"strings"
	"testing"
)

func TestTextPreeditLargeSelectionCopiesOnlyEndpoints(t *testing.T) {
	model := NewModel("head\n" + strings.Repeat("body\n", 200000) + "tail")
	p := NewProjection(model, Selection{2, model.Len() - 2})
	p.Update("中", 3)
	if p.display != "he中il" || len(p.prefix)+len(p.suffix) != 4 || len(p.lines) != 1 {
		t.Fatal("large selection copied/shaped hidden document")
	}
}

// 超大单行预编辑只复制调用方请求的范围，不复制其余前缀、后缀。
func TestProjectionLongSingleLineReadsRanges(t *testing.T) {
	model := NewModel(strings.Repeat("a", 2<<20) + "end")
	at := model.Len() / 2
	p := NewProjection(model, Selection{at, at})
	p.Update("中\n文", 7)
	if len(p.display) != 0 || len(p.prefix) != 0 || len(p.suffix) != 0 || p.ParagraphCount() != 2 {
		t.Fatal("长行投影发生整段物化")
	}
	got, err := p.Slice(model, Range{at - 2, at + len(p.Text()) + 2})
	if err != nil || got != "aa中\n文aa" {
		t.Fatalf("投影范围读取：%q/%v", got, err)
	}
	if p.LineAt(model, at+4) != 1 || p.LineRange(model, 1).Start != at+4 {
		t.Fatal("预编辑 LF 范围映射错误")
	}
}
