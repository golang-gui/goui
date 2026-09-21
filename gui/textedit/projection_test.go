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
