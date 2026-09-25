package codec

import (
	"container/heap"
	"math"
)

type mergeNode struct {
	offset     int
	prev, next int
	rank       uint
	heapIndex  int
}

// Nodes stay at their original byte offsets. The heap breaks equal-rank ties
// left-to-right, exactly as the original BPE scan does.
type mergeHeap struct {
	nodes []mergeNode
	items []*mergeNode
}

func (h mergeHeap) Len() int { return len(h.items) }

func (h mergeHeap) Less(i, j int) bool {
	a, b := h.items[i], h.items[j]
	if a.rank == b.rank {
		return a.offset < b.offset
	}
	return a.rank < b.rank
}

func (h mergeHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].heapIndex = i
	h.items[j].heapIndex = j
}

func (h *mergeHeap) Push(value any) {
	node := value.(*mergeNode)
	node.heapIndex = len(h.items)
	h.items = append(h.items, node)
}

func (h *mergeHeap) Pop() any {
	last := len(h.items) - 1
	node := h.items[last]
	h.items[last] = nil
	h.items = h.items[:last]
	node.heapIndex = -1
	return node
}

func (c *Codec) mergePairsHeap(piece string) []part {
	h := &mergeHeap{
		nodes: make([]mergeNode, len(piece)),
		items: make([]*mergeNode, 0, len(piece)),
	}
	for i := range h.nodes {
		h.nodes[i] = mergeNode{offset: i, prev: i - 1, next: i + 1, rank: math.MaxUint, heapIndex: -1}
		if i+1 < len(piece) {
			if rank, ok := c.vocabulary[piece[i:i+2]]; ok {
				h.nodes[i].rank = rank
				h.nodes[i].heapIndex = len(h.items)
				h.items = append(h.items, &h.nodes[i])
			}
		}
	}
	heap.Init(h)
	remaining := len(piece)
	for h.Len() > 0 {
		left := heap.Pop(h).(*mergeNode).offset
		right := h.nodes[left].next
		if index := h.nodes[right].heapIndex; index >= 0 {
			heap.Remove(h, index)
		}
		next := h.nodes[right].next
		h.nodes[left].next = next
		if next < len(piece) {
			h.nodes[next].prev = left
		}
		remaining--

		// Only the two pairs touching the merged token can change rank.
		for _, i := range [2]int{left, h.nodes[left].prev} {
			if i < 0 {
				continue
			}
			rank := uint(math.MaxUint)
			if neighbor := h.nodes[i].next; neighbor < len(piece) {
				if value, ok := c.vocabulary[piece[i:h.nodes[neighbor].next]]; ok {
					rank = value
				}
			}
			h.nodes[i].rank = rank
			index := h.nodes[i].heapIndex
			switch {
			case rank == math.MaxUint && index >= 0:
				heap.Remove(h, index)
			case rank != math.MaxUint && index >= 0:
				heap.Fix(h, index)
			case rank != math.MaxUint:
				heap.Push(h, &h.nodes[i])
			}
		}
	}
	parts := make([]part, 0, remaining+1)
	for i := 0; i < len(piece); i = h.nodes[i].next {
		parts = append(parts, part{offset: i, rank: math.MaxUint})
	}
	return append(parts, part{offset: len(piece), rank: math.MaxUint})
}
