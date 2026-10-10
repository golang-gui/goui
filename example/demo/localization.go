package main

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/ui"
)

// Set is called by BindText or an explicit application operation: either hands
// ownership to the user, even when the assigned value equals the sample.
type sampleInput struct {
	state  ui.State[string]
	edited bool
}

func newSampleInput(text string) sampleInput { return sampleInput{state: ui.MakeState(text)} }
func (s *sampleInput) Get() string           { return s.state.Get() }
func (s *sampleInput) Set(text string)       { s.edited = true; s.state.Set(text) }
func (s *sampleInput) translate(text string) {
	if !s.edited {
		s.state.Set(text)
	}
}

// Revision records the last application-owned sample, not its text. Undoing
// back to the initial contents must not make user-owned history replaceable.
func translateSample(model *ui.TextModel, revision *uint64, text string) {
	if model.Revision() == *revision {
		model.SetText(text)
		*revision = model.Revision()
	}
}

func documentSample(id string) string {
	return fmt.Sprintf(resource.DocumentSample(), strings.TrimPrefix(id, "document-"))
}

// Keep the models, stable IDs, current order and user-owned data in place.
// ui.Run already requests a rebuild when the language changes.
func (d *demo) translateUI() {
	if d.language == mlang.Language() {
		return
	}
	d.language = mlang.Language()
	d.input.translate(resource.InputSample())
	d.baselineInput.translate(resource.BaselineSample())
	d.splitInput.translate(resource.SplitInputSample())
	translateSample(d.text, &d.textRevision, resource.EditorSample())
	translateSample(d.inspection, &d.inspectionRevision, resource.InspectionSample())
	for id, model := range d.documents {
		revision := d.documentRevisions[id]
		translateSample(model, &revision, documentSample(id))
		d.documentRevisions[id] = revision
	}
	labels := []string{resource.DefaultOption(), resource.CustomOption(), resource.DisabledOption(), resource.LastOption()}
	for i, label := range labels {
		item := d.choices.ItemAt(i)
		if item.Text != label {
			item.Text = label
			d.choices.Set(i, item)
		}
	}
	d.tree.Set("project", resource.SampleProject())
	d.list.Modify(func(rows []string) []string {
		for i := range rows {
			rows[i] = fmt.Sprintf(resource.ListRow(), i+1)
		}
		return rows
	})
	d.table.translate()
}
