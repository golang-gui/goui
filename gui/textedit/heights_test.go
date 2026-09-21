package textedit

import (
	"math/rand"
	"testing"
)

func TestTextHeightsSplicesAgainstArray(t *testing.T) {
	var h HeightIndex
	h.Reset(10000, 20)
	if h.root.run != 10000 || h.root.left != nil || h.root.right != nil {
		t.Fatal("unmeasured document should be a single run")
	}
	ref := make([]float32, 10000)
	for i := range ref {
		ref[i] = 20
	}
	rng := rand.New(rand.NewSource(718))
	for op := 0; op < 2000; op++ {
		i := rng.Intn(len(ref))
		if op%3 != 0 {
			v := float32(1+rng.Intn(100))/2 + 1
			h.Set(i, v)
			ref[i] = v
		} else {
			remove := min(len(ref)-i, rng.Intn(5))
			insert := 1 + rng.Intn(4)
			h.Splice(i, remove, insert, 20)
			next := make([]float32, 0, len(ref)-remove+insert)
			next = append(next, ref[:i]...)
			for n := 0; n < insert; n++ {
				next = append(next, 20)
			}
			ref = append(next, ref[i+remove:]...)
		}
		var sum float32
		probe := rng.Intn(len(ref))
		for j, v := range ref {
			if j == probe {
				if got := h.Top(j); got != sum {
					t.Fatalf("op %d Top %d: %g want %g", op, j, got, sum)
				}
				if got := h.At(sum + v/2); got != j {
					t.Fatalf("op %d At: %d want %d", op, got, j)
				}
			}
			sum += v
		}
		if heightCount(h.root) != len(ref) || h.Total() != sum {
			t.Fatalf("op %d count/Total inconsistent", op)
		}
		checkHeightNode(t, h.root)
	}
	if h.At(-1) != 0 || h.At(h.Total()+100) != len(ref)-1 {
		t.Fatal("out-of-range lookup not clamped")
	}
}

func checkHeightNode(t *testing.T, n *textHeightNode) {
	t.Helper()
	if n == nil {
		return
	}
	if n.run <= 0 || n.count != heightCount(n.left)+n.run+heightCount(n.right) || n.sum != heightSum(n.left)+float64(n.run)*n.height+heightSum(n.right) {
		t.Fatal("height aggregate invariant")
	}
	for _, child := range []*textHeightNode{n.left, n.right} {
		if child != nil && child.priority > n.priority {
			t.Fatal("height heap invariant")
		}
		checkHeightNode(t, child)
	}
}

func TestTextHeightsSparseMeasurementDepth(t *testing.T) {
	var h HeightIndex
	h.Reset(20000, 20)
	for i := 0; i < 20000; i += 2 {
		h.Set(i, 21)
	}
	var depth func(*textHeightNode, int)
	depth = func(n *textHeightNode, d int) {
		if n == nil {
			return
		}
		if d > 100 {
			t.Fatal("sparse measurements degenerated height index into a chain")
		}
		depth(n.left, d+1)
		depth(n.right, d+1)
	}
	depth(h.root, 0)
}
