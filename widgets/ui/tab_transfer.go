package ui

import (
	"fmt"

	"github.com/golang-gui/goui/core/signal"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// TabTransfer reports a committed node move. PageID retains the page's identity;
// window/view IDs identify the two declaration collections to update.
type TabTransfer struct {
	PageID                   string
	SourceWindow, SourceView string
	TargetWindow, TargetView string
	Index                    int
}

// TabDetachRequest supplies the page ID alongside the imperative preparation
// request. Declare the empty destination before calling TransferTo. OnTransfer
// updates both declaration collections only after successful node adoption.
type TabDetachRequest struct {
	*widgets.TabDetachRequest
	PageID string
}

type tabBarState struct {
	handles            signal.Handles
	onError            func(error)
	onDetach           func(*TabDetachRequest, *bool)
	onContextMenuError func(error)
	contextMenu        func(string) []*baseui.MenuItemView
}

func pageID(page *widgets.TabPage) string {
	if page == nil {
		return ""
	}
	return page.ID()
}

func (s *tabViewState) transfer(request *widgets.TabTransferRequest) {
	request.Handled = true // mixed GUI/UI ownership must never fall back to GUI
	if s.coordinator == nil || s.updating || request.Source != s.pages.view {
		request.Err = fmt.Errorf("widgets/ui: transfer needs live source UI ownership")
		return
	}
	if request.Source == request.Target {
		request.Err = request.Commit()
		return
	}
	if pageID(request.Page) == "" || s.onTransfer == nil {
		request.Err = fmt.Errorf("widgets/ui: transfer needs a page ID and source OnTransfer handler")
		return
	}
	fromWindow, toWindow := request.Source.Window(), request.Target.Window()
	if fromWindow == nil || toWindow == nil || fromWindow.ID() == "" || toWindow.ID() == "" || request.Source.ID() == "" || request.Target.ID() == "" {
		request.Err = fmt.Errorf("widgets/ui: transferred views and windows need IDs")
		return
	}
	change := TabTransfer{PageID: request.Page.ID(), SourceWindow: fromWindow.ID(), SourceView: request.Source.ID(),
		TargetWindow: toWindow.ID(), TargetView: request.Target.ID(), Index: request.Index}
	onTransfer := s.onTransfer
	request.Err = s.coordinator.TransferChild(request.Page, request.Target, request.Commit, func() { onTransfer(change) })
}
