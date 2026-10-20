package gui

import (
	"time"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/typography"
)

// DeferTextLayout requests a deferred box change when the backend supports it.
// The old layout remains usable until completion. All requests on one Root
// share a frame budget. ready runs on the GUI thread after publication (or
// failure); cancelled requests do not call it. The caller must cancel before
// changing the layout's measurement formatting. Unmount/destruction cancels
// automatically. A newer request for the same layout supersedes the old one.
//
// If scheduled is false, no change was made and the caller should use SetSize.
// The returned cancellation function is idempotent. The caller owns layout.
func DeferTextLayout(owner Widget, layout typography.TextLayout, width, height float32, ready func(error)) (cancel func(), scheduled bool) {
	native, ok := layout.(typography.ReflowLayout)
	root := owner.Root()
	if !ok || root == nil || owner.base().destroyed {
		return nil, false
	}
	q := textReflows[root]
	if q != nil {
		if q.running != nil && q.running.layout == layout {
			q.running.cancel()
		}
		for _, job := range append([]*textReflowJob(nil), q.jobs...) {
			if job.layout == layout {
				job.cancel()
			}
		}
	}
	if !native.Reflow(width, height) {
		return nil, false
	}
	q = textReflows[root]
	if q == nil {
		q = &textReflowQueue{root: root}
		textReflows[root] = q
		q.frame = root.ConnectFrame(q.advance)
	}
	job := &textReflowJob{queue: q, owner: owner, layout: native, ready: ready}
	q.jobs = append(q.jobs, job)
	job.unmount = owner.ConnectUnmount(job.cancel)
	job.destroy = layout.ConnectDestroy(func() { job.release(false) })
	owner.RequestPaint()
	return job.cancel, true
}

type textReflowJob struct {
	queue            *textReflowQueue
	owner            Widget
	layout           typography.ReflowLayout
	ready            func(error)
	unmount, destroy signal.Handle
	cancelled        bool
}

func (j *textReflowJob) cancel() { j.release(true) }

func (j *textReflowJob) release(cancelNative bool) {
	if j.cancelled {
		return
	}
	j.cancelled = true
	if cancelNative {
		j.layout.CancelReflow()
	}
	if j.unmount != nil {
		j.unmount.Disconnect()
	}
	if j.destroy != nil {
		j.destroy.Disconnect()
	}
	j.ready = nil
	if !j.queue.advancing {
		j.queue.prune()
	}
}

type textReflowQueue struct {
	root      Root
	frame     signal.Handle
	jobs      []*textReflowJob
	running   *textReflowJob
	advancing bool
}

var textReflows = make(map[Root]*textReflowQueue)

func (q *textReflowQueue) prune() {
	live := q.jobs[:0]
	for _, j := range q.jobs {
		if !j.cancelled {
			live = append(live, j)
		}
	}
	clear(q.jobs[len(live):])
	q.jobs = live
	if len(q.jobs) == 0 && q.running == nil {
		q.frame.Disconnect()
		if textReflows[q.root] == q {
			delete(textReflows, q.root)
		}
	}
}

func (q *textReflowQueue) advance(time.Time) {
	q.advancing = true
	deadline := time.Now().Add(2 * time.Millisecond)
	idle := 0
	for len(q.jobs) > 0 && idle < len(q.jobs) && time.Now().Before(deadline) {
		job := q.jobs[0]
		q.jobs[0] = nil
		q.jobs = q.jobs[1:]
		if job.cancelled {
			continue
		}
		if job.owner.Root() != q.root || job.owner.base().destroyed {
			job.cancel()
			continue
		}
		if !visibleInTree(job.owner) {
			q.jobs = append(q.jobs, job)
			idle++
			continue
		}
		idle = 0
		q.running = job
		done, err := job.layout.AdvanceReflow(minTimeReflow(deadline, time.Now().Add(250*time.Microsecond)))
		if !job.cancelled && done {
			ready := job.ready
			job.release(false)
			if ready != nil {
				ready(err)
			}
		}
		q.running = nil
		if !job.cancelled {
			q.jobs = append(q.jobs, job)
		}
	}
	q.advancing = false
	q.prune()
	for _, job := range q.jobs {
		if !job.cancelled && visibleInTree(job.owner) {
			_ = q.root.RequestPaint()
			break
		}
	}
}

func minTimeReflow(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
