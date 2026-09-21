package textedit

// HeightIndex is an implicit treap of equal-height paragraph runs. Unmeasured
// paragraphs occupy one run, not one native layout or one Widget per line.
// Splitting/replacing paragraphs and changing a measured height touch only the
// search paths. Float64 sums avoid losing sub-DIP heights in long documents.
type HeightIndex struct {
	root *textHeightNode
	seed uint64
}

// Count returns the number of logical paragraphs represented by the index.
func (h *HeightIndex) Count() int { return heightCount(h.root) }

type textHeightNode struct {
	left, right *textHeightNode
	priority    uint64
	run, count  int
	height, sum float64
}

func heightCount(n *textHeightNode) int {
	if n == nil {
		return 0
	}
	return n.count
}

func heightSum(n *textHeightNode) float64 {
	if n == nil {
		return 0
	}
	return n.sum
}

func (n *textHeightNode) update() {
	n.count = heightCount(n.left) + n.run + heightCount(n.right)
	n.sum = heightSum(n.left) + float64(n.run)*n.height + heightSum(n.right)
}

func (h *HeightIndex) node(count int, height float64) *textHeightNode {
	if count <= 0 {
		return nil
	}
	// SplitMix64: deterministic, local to this index, no global RNG state.
	h.seed += 0x9e3779b97f4a7c15
	z := h.seed
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	n := &textHeightNode{run: count, height: max(1, height), priority: z}
	n.update()
	return n
}

func mergeHeights(a, b *textHeightNode) *textHeightNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.priority > b.priority {
		a.right = mergeHeights(a.right, b)
		a.update()
		return a
	}
	b.left = mergeHeights(a, b.left)
	b.update()
	return b
}

func (h *HeightIndex) split(n *textHeightNode, index int) (*textHeightNode, *textHeightNode) {
	if n == nil {
		return nil, nil
	}
	leftCount := heightCount(n.left)
	if index < leftCount {
		a, b := h.split(n.left, index)
		n.left = b
		n.update()
		return a, n
	}
	if index > leftCount+n.run {
		a, b := h.split(n.right, index-leftCount-n.run)
		n.right = a
		n.update()
		return n, b
	}
	if index == leftCount {
		a := n.left
		n.left = nil
		n.update()
		return a, n
	}
	if index == leftCount+n.run {
		b := n.right
		n.right = nil
		n.update()
		return n, b
	}
	// Preserve the heap priority of both fragments so ancestor links remain
	// valid. Equal priorities are legal; future inserted nodes use new values.
	a := &textHeightNode{left: n.left, run: index - leftCount, height: n.height, priority: n.priority}
	b := &textHeightNode{right: n.right, run: n.run - a.run, height: n.height, priority: n.priority}
	a.update()
	b.update()
	return a, b
}

// Reset replaces the index with count paragraphs of the estimated DIP height.
func (h *HeightIndex) Reset(count int, estimate float32) {
	h.root = h.node(count, float64(estimate))
}

// Splice replaces a valid paragraph range with inserted estimated paragraphs.
func (h *HeightIndex) Splice(start, removed, inserted int, estimate float32) {
	a, rest := h.split(h.root, start)
	_, b := h.split(rest, removed)
	h.root = mergeHeights(mergeHeights(a, h.node(inserted, float64(estimate))), b)
}

// Set updates a paragraph's measured DIP height; out-of-range indexes are ignored.
func (h *HeightIndex) Set(index int, height float32) {
	if index < 0 || index >= heightCount(h.root) {
		return
	}
	h.Splice(index, 1, 1, height)
}

// Total returns the document height in DIP.
func (h *HeightIndex) Total() float32 { return float32(heightSum(h.root)) }

// Top returns the DIP height preceding index, clamped to [0, Count()].
func (h *HeightIndex) Top(index int) float32 {
	index = min(max(index, 0), heightCount(h.root))
	n, sum := h.root, float64(0)
	for n != nil {
		leftCount := heightCount(n.left)
		if index < leftCount {
			n = n.left
			continue
		}
		sum += heightSum(n.left)
		inRun := min(index-leftCount, n.run)
		sum += float64(inRun) * n.height
		index -= leftCount + inRun
		if index == 0 {
			break
		}
		n = n.right
	}
	return float32(sum)
}

// At returns the containing paragraph, clamping points outside the document.
func (h *HeightIndex) At(y float32) int {
	n, index, remaining := h.root, 0, max(0, float64(y))
	for n != nil {
		leftSum := heightSum(n.left)
		if remaining < leftSum {
			n = n.left
			continue
		}
		remaining -= leftSum
		index += heightCount(n.left)
		runSum := float64(n.run) * n.height
		if remaining < runSum {
			return index + int(remaining/n.height)
		}
		remaining -= runSum
		index += n.run
		n = n.right
	}
	return max(0, heightCount(h.root)-1)
}
