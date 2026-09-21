package textedit

import (
	"bytes"
	"strings"
)

// textPieceMaxSize bounds both loaded and inserted blocks. A line query scans
// at most one bounded block after descending through the subtree aggregates.
const textPieceMaxSize = 64 * 1024

// A block only grows at its tail; existing bytes never change. Pieces and
// history directly own the blocks they reference, without a document-wide
// buffer registry retaining deleted content. Losing the last reference makes
// a block eligible for normal Go garbage collection.
type textBlock struct{ data []byte }

// pieceRef identifies an immutable byte range within a bounded block.
type pieceRef struct {
	block  *textBlock
	start  int
	length int
}

// pieceNode is an AVL node over one piece. The subtree totals (size, lineFeed)
// let byte-offset and line queries descend in logarithmic time; heights keep
// the tree balanced.
type pieceNode struct {
	parent   *pieceNode
	left     *pieceNode
	right    *pieceNode
	piece    pieceRef
	lfCount  int   // line feeds within this piece
	height   int32 // AVL height, leaf = 1
	size     int   // bytes in this subtree
	lineFeed int   // line feeds in this subtree
}

// pieceTree stores a document as the ordered set of its pieces (the VSCode
// piece tree idea, in Go and byte-based): initial content is carved into
// bounded pieces, edits append to reusable block tails and insert/remove pieces,
// and nothing ever copies or shifts the whole document.
type pieceTree struct {
	root    *pieceNode
	lfCache pieceTreeLFCache
}

// pieceTreeLFCache remembers one found line feed so sequential line iteration
// (line k followed by line k+1) continues scanning instead of restarting from
// the tree root.
type pieceTreeLFCache struct {
	node      *pieceNode
	nodeStart int // absolute offset of node's piece
	idxInNode int // byte index of the cached line feed within the piece
	index     int // ordinal of the cached line feed
	offset    int // absolute offset of the cached line feed
}

func nodeSize(n *pieceNode) int {
	if n == nil {
		return 0
	}
	return n.size
}

func nodeLF(n *pieceNode) int {
	if n == nil {
		return 0
	}
	return n.lineFeed
}

func nodeHeight(n *pieceNode) int32 {
	if n == nil {
		return 0
	}
	return n.height
}

func (n *pieceNode) update() {
	n.size = nodeSize(n.left) + n.piece.length + nodeSize(n.right)
	n.lineFeed = nodeLF(n.left) + n.lfCount + nodeLF(n.right)
	n.height = max(nodeHeight(n.left), nodeHeight(n.right)) + 1
}

func (t *pieceTree) length() int {
	return nodeSize(t.root)
}

func (t *pieceTree) lineCount() int {
	return nodeLF(t.root) + 1
}

func (t *pieceTree) buffer(ref pieceRef) []byte {
	return ref.block.data
}

func (t *pieceTree) leftmost() *pieceNode {
	n := t.root
	for n != nil && n.left != nil {
		n = n.left
	}
	return n
}

func (t *pieceTree) successor(n *pieceNode) *pieceNode {
	if n.right != nil {
		n = n.right
		for n.left != nil {
			n = n.left
		}
		return n
	}
	for n.parent != nil && n.parent.right == n {
		n = n.parent
	}
	return n.parent
}

func (t *pieceTree) rotateLeft(x *pieceNode) *pieceNode {
	y := x.right
	x.right = y.left
	if y.left != nil {
		y.left.parent = x
	}
	y.parent = x.parent
	if x.parent == nil {
		t.root = y
	} else if x.parent.left == x {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.left = x
	x.parent = y
	x.update()
	y.update()
	return y
}

func (t *pieceTree) rotateRight(x *pieceNode) *pieceNode {
	y := x.left
	x.left = y.right
	if y.right != nil {
		y.right.parent = x
	}
	y.parent = x.parent
	if x.parent == nil {
		t.root = y
	} else if x.parent.left == x {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.right = x
	x.parent = y
	x.update()
	y.update()
	return y
}

// rebalance recomputes aggregates and restores the AVL invariant from node up
// to the root.
func (t *pieceTree) rebalance(node *pieceNode) {
	for node != nil {
		node.update()
		switch balance := nodeHeight(node.left) - nodeHeight(node.right); {
		case balance > 1:
			if nodeHeight(node.left.left) < nodeHeight(node.left.right) {
				t.rotateLeft(node.left)
			}
			node = t.rotateRight(node)
		case balance < -1:
			if nodeHeight(node.right.right) < nodeHeight(node.right.left) {
				t.rotateRight(node.right)
			}
			node = t.rotateLeft(node)
		}
		node = node.parent
	}
}

// nodeAt returns the node whose piece contains the byte at offset, the absolute
// offset of that piece and offset's index within it. offset must be within
// [0, length).
func (t *pieceTree) nodeAt(offset int) (*pieceNode, int, int) {
	node := t.root
	start := 0
	for {
		if leftSize := nodeSize(node.left); offset < start+leftSize {
			node = node.left
			continue
		} else {
			start += leftSize
		}
		if offset < start+node.piece.length {
			return node, start, offset - start
		}
		start += node.piece.length
		node = node.right
	}
}

// findLF returns the absolute byte offset of the n-th (zero-based) line feed;
// ok is false when the document has no n-th line feed.
func (t *pieceTree) findLF(n int) (int, bool) {
	if n < 0 || n >= nodeLF(t.root) {
		return 0, false
	}
	if t.lfCache.node != nil {
		if n == t.lfCache.index {
			return t.lfCache.offset, true
		}
		if n == t.lfCache.index+1 {
			return t.findLFForward(n)
		}
	}
	return t.findLFDescend(n)
}

func (t *pieceTree) findLFDescend(n int) (int, bool) {
	node, start, before := t.root, 0, 0
	for node != nil {
		leftLF := nodeLF(node.left)
		if n < before+leftLF {
			node = node.left
			continue
		}
		before += leftLF
		start += nodeSize(node.left)
		if n < before+node.lfCount {
			idx := pieceLFIndex(t.buffer(node.piece), node.piece, n-before)
			if idx >= 0 {
				t.cacheLF(n, node, start, idx)
				return start + idx, true
			}
		}
		before += node.lfCount
		start += node.piece.length
		node = node.right
	}
	return 0, false
}

// findLFForward continues scanning from the cached line feed; it makes
// sequential line queries cost the bytes actually scanned, not a full descent.
func (t *pieceTree) findLFForward(n int) (int, bool) {
	cache := t.lfCache
	// seen is the ordinal the next scanned line feed will have: the cached
	// one is already behind us.
	node, start, idx, seen := cache.node, cache.nodeStart, cache.idxInNode+1, cache.index+1
	for node != nil {
		buf := t.buffer(node.piece)
		for i := idx; i < node.piece.length; i++ {
			if buf[node.piece.start+i] != '\n' {
				continue
			}
			if seen == n {
				t.cacheLF(n, node, start, i)
				return start + i, true
			}
			seen++
		}
		start += node.piece.length
		idx = 0
		node = t.successor(node)
	}
	return 0, false
}

func (t *pieceTree) cacheLF(n int, node *pieceNode, nodeStart, idxInNode int) {
	t.lfCache = pieceTreeLFCache{
		node: node, nodeStart: nodeStart, idxInNode: idxInNode,
		index: n, offset: nodeStart + idxInNode,
	}
}

// pieceLFIndex returns the byte index of the n-th line feed within the piece,
// or -1 when the piece holds fewer than n+1 line feeds.
func pieceLFIndex(buf []byte, p pieceRef, n int) int {
	count := 0
	for i := 0; i < p.length; i++ {
		if buf[p.start+i] == '\n' {
			if count == n {
				return i
			}
			count++
		}
	}
	return -1
}

// splitAt makes offset a piece boundary by splitting the piece containing it.
// It is a no-op at the document ends or at existing boundaries.
func (t *pieceTree) splitAt(offset int) {
	if offset <= 0 || offset >= t.length() {
		return
	}
	node, _, idx := t.nodeAt(offset)
	if idx == 0 {
		return
	}
	leftLF := pieceLFCount(t.buffer(node.piece), node.piece, idx)
	right := &pieceNode{
		piece:   pieceRef{block: node.piece.block, start: node.piece.start + idx, length: node.piece.length - idx},
		lfCount: node.lfCount - leftLF,
		height:  1,
	}
	right.size, right.lineFeed = right.piece.length, right.lfCount
	node.lfCount = leftLF
	node.piece.length = idx
	t.insertSuccessor(node, right)
}

// pieceLFCount counts line feeds within the first length bytes of the piece.
func pieceLFCount(buf []byte, p pieceRef, length int) int {
	return bytes.Count(buf[p.start:p.start+length], []byte{'\n'})
}

// insertSuccessor links node into the tree directly after the in-order node
// `after`; a nil after inserts at the leftmost position.
func (t *pieceTree) insertSuccessor(after, node *pieceNode) {
	node.parent, node.left, node.right = nil, nil, nil
	if after == nil {
		if t.root == nil {
			t.root = node
			node.update()
			return
		}
		after = t.root
		for after.left != nil {
			after = after.left
		}
		after.left = node
		node.parent = after
	} else if after.right == nil {
		after.right = node
		node.parent = after
	} else {
		x := after.right
		for x.left != nil {
			x = x.left
		}
		x.left = node
		node.parent = x
	}
	t.rebalance(node)
}

// insertPiecesAt inserts the document-ordered refs as pieces at offset.
func (t *pieceTree) insertPiecesAt(offset int, refs []pieceRef) {
	if len(refs) == 0 {
		return
	}
	t.lfCache = pieceTreeLFCache{}
	t.splitAt(offset)
	var anchor *pieceNode
	if offset > 0 {
		anchor, _, _ = t.nodeAt(offset - 1)
	}
	for _, ref := range refs {
		if ref.length == 0 {
			continue
		}
		lf := pieceLFCount(t.buffer(ref), ref, ref.length)
		node := &pieceNode{piece: ref, lfCount: lf, height: 1, size: ref.length, lineFeed: lf}
		t.insertSuccessor(anchor, node)
		anchor = node
	}
}

// insertText reuses the preceding piece's block tail where possible. There is
// no separate writer reference keeping an otherwise unused block alive. Large
// pastes split into bounded blocks just like loaded content.
func (t *pieceTree) insertText(offset int, text string) []pieceRef {
	t.lfCache = pieceTreeLFCache{}
	t.splitAt(offset)
	var anchor *pieceNode
	if offset > 0 {
		anchor, _, _ = t.nodeAt(offset - 1)
	}
	var refs []pieceRef
	for len(text) != 0 {
		var block *textBlock
		if anchor != nil {
			b := anchor.piece.block
			if anchor.piece.start+anchor.piece.length == len(b.data) && len(b.data) < textPieceMaxSize {
				block = b
			}
		}
		if block == nil {
			block = &textBlock{}
		}
		start := len(block.data)
		count := min(len(text), textPieceMaxSize-start)
		if start+count > cap(block.data) {
			capacity := min(textPieceMaxSize, max(32, max(start+count, 2*cap(block.data))))
			data := make([]byte, start, capacity)
			copy(data, block.data)
			block.data = data
		}
		block.data = append(block.data, text[:count]...)
		ref := pieceRef{block: block, start: start, length: count}
		lf := strings.Count(text[:count], "\n")
		if anchor != nil && anchor.piece.block == block {
			anchor.piece.length += count
			anchor.lfCount += lf
			t.rebalance(anchor)
		} else {
			node := &pieceNode{piece: ref, lfCount: lf, height: 1, size: count, lineFeed: lf}
			t.insertSuccessor(anchor, node)
			anchor = node
		}
		refs = append(refs, ref)
		text = text[count:]
	}
	return refs
}

// deleteRange removes the bytes in [start, end) and returns their pieces in
// document order.
func (t *pieceTree) deleteRange(start, end int) []pieceRef {
	if start >= end {
		return nil
	}
	t.lfCache = pieceTreeLFCache{}
	t.splitAt(start)
	t.splitAt(end)
	var removed []pieceRef
	node, nodeStart, _ := t.nodeAt(start)
	for node != nil && nodeStart+node.piece.length <= end {
		next := t.successor(node)
		nextStart := nodeStart + node.piece.length
		removed = append(removed, node.piece)
		t.removeNode(node)
		node, nodeStart = next, nextStart
	}
	return removed
}

// transplant replaces the subtree rooted at u with the one rooted at v.
func (t *pieceTree) transplant(u, v *pieceNode) {
	if u.parent == nil {
		t.root = v
	} else if u.parent.left == u {
		u.parent.left = v
	} else {
		u.parent.right = v
	}
	if v != nil {
		v.parent = u.parent
	}
}

// removeNode unlinks a single node from the tree, preserving in-order content.
func (t *pieceTree) removeNode(z *pieceNode) {
	var fix *pieceNode
	switch {
	case z.left == nil:
		fix = z.parent
		t.transplant(z, z.right)
	case z.right == nil:
		fix = z.parent
		t.transplant(z, z.left)
	default:
		y := z.right
		for y.left != nil {
			y = y.left
		}
		if y.parent == z {
			t.transplant(z, y)
			y.left = z.left
			z.left.parent = y
			fix = y
		} else {
			p := y.parent
			t.transplant(y, y.right)
			y.right = z.right
			z.right.parent = y
			t.transplant(z, y)
			y.left = z.left
			z.left.parent = y
			fix = p
		}
		y.update()
	}
	if fix != nil {
		t.rebalance(fix)
	}
}

// build replaces the tree with separately allocated, bounded blocks. Old
// references remain valid until their owners release them.
func (t *pieceTree) build(text string) {
	t.root = nil
	t.lfCache = pieceTreeLFCache{}
	if len(text) == 0 {
		return
	}
	var nodes []*pieceNode
	for start := 0; start < len(text); {
		end := min(start+textPieceMaxSize, len(text))
		block := &textBlock{data: []byte(text[start:end])}
		n := &pieceNode{
			piece:  pieceRef{block: block, length: end - start},
			height: 1,
		}
		n.lfCount = pieceLFCount(block.data, n.piece, n.piece.length)
		n.size, n.lineFeed = n.piece.length, n.lfCount
		nodes = append(nodes, n)
		start = end
	}
	t.root = buildBalancedNodes(nodes, nil)
}

func buildBalancedNodes(nodes []*pieceNode, parent *pieceNode) *pieceNode {
	var build func(lo, hi int, parent *pieceNode) *pieceNode
	build = func(lo, hi int, parent *pieceNode) *pieceNode {
		if lo >= hi {
			return nil
		}
		mid := (lo + hi) / 2
		n := nodes[mid]
		n.parent = parent
		n.left = build(lo, mid, n)
		n.right = build(mid+1, hi, n)
		n.update()
		return n
	}
	return build(0, len(nodes), parent)
}

// text materializes the whole document.
func (t *pieceTree) text() string {
	var b strings.Builder
	b.Grow(t.length())
	var walk func(n *pieceNode)
	walk = func(n *pieceNode) {
		if n == nil {
			return
		}
		walk(n.left)
		b.Write(t.buffer(n.piece)[n.piece.start : n.piece.start+n.piece.length])
		walk(n.right)
	}
	walk(t.root)
	return b.String()
}

// equalText reports whether the document equals s without materializing it.
func (t *pieceTree) equalText(s string) bool {
	if t.length() != len(s) {
		return false
	}
	pos := 0
	for node := t.leftmost(); node != nil; node = t.successor(node) {
		piece := t.buffer(node.piece)[node.piece.start : node.piece.start+node.piece.length]
		if s[pos:pos+len(piece)] != string(piece) {
			return false
		}
		pos += len(piece)
	}
	return true
}

// sliceRange materializes the bytes in [start, end).
func (t *pieceTree) sliceRange(start, end int) string {
	var b strings.Builder
	b.Grow(end - start)
	node, nodeStart, _ := t.nodeAt(start)
	for node != nil && nodeStart < end {
		pieceFrom := max(start, nodeStart)
		pieceTo := min(nodeStart+node.piece.length, end)
		if pieceTo > pieceFrom {
			buf := t.buffer(node.piece)
			b.Write(buf[node.piece.start+pieceFrom-nodeStart : node.piece.start+pieceTo-nodeStart])
		}
		nodeStart += node.piece.length
		node = t.successor(node)
	}
	return b.String()
}

// textOfRefs materializes the concatenation of refs.
func (t *pieceTree) textOfRefs(refs []pieceRef) string {
	var b strings.Builder
	for _, ref := range refs {
		b.Write(t.buffer(ref)[ref.start : ref.start+ref.length])
	}
	return b.String()
}

func pieceRefsLen(refs []pieceRef) int {
	total := 0
	for _, ref := range refs {
		total += ref.length
	}
	return total
}

// Coalesce history ranges as well as document pieces. One long typing group
// must not accumulate one history reference per key, and deleting backwards
// must not copy a growing reference slice on every key.
func appendPieceRefs(refs, suffix []pieceRef) []pieceRef {
	for _, ref := range suffix {
		if len(refs) != 0 {
			last := &refs[len(refs)-1]
			if last.block == ref.block && last.start+last.length == ref.start {
				last.length += ref.length
				continue
			}
		}
		refs = append(refs, ref)
	}
	return refs
}

func prependPieceRefs(refs, prefix []pieceRef) []pieceRef {
	if len(prefix) != 0 && len(refs) != 0 {
		last := prefix[len(prefix)-1]
		first := &refs[0]
		if last.block == first.block && last.start+last.length == first.start {
			first.start = last.start
			first.length += last.length
			prefix = prefix[:len(prefix)-1]
		}
	}
	if len(prefix) == 0 {
		return refs
	}
	return append(prefix, refs...)
}
