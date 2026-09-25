# Local tokenizer patch

This directory contains github.com/tiktoken-go/tokenizer v0.6.2, with its
original MIT license, model mappings, regular expressions and vocabularies.

`codec/merge_heap.go` replaces the repeated full scan and slice movement for
pieces longer than 128 bytes with an indexed priority queue and linked byte
offsets. Equal-rank merges still choose the leftmost pair. Short pieces use the
original algorithm. Token IDs, not just counts, are regression-tested against
the original algorithm for every supported encoding.

The queue has at most one entry per live pair. Long-piece merging uses
O(n log n) queue work and O(n) space instead of quadratic scans and copies.
No text is truncated, estimated or retained in a request cache.

From the repository root:

    go test github.com/tiktoken-go/tokenizer/...
    go test github.com/tiktoken-go/tokenizer/codec -run '^$' -bench BenchmarkLongPiece -benchmem
