package events

// MenuEvent reports a native selection after menu tracking has ended.
// Item identifies the selected action binding in the receiving window.
type MenuEvent struct{ Item uint64 }

func (MenuEvent) Type() EventType { return MenuActivate }
func (MenuEvent) isEvent()        {}
