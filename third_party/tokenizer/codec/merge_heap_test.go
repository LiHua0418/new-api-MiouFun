package codec

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLongPieceTokenIDsMatchOriginal(t *testing.T) {
	codecs := []*Codec{NewCl100kBase(), NewO200kBase(), NewR50kBase(), NewP50kBase(), NewP50kEdit()}
	cases := []struct {
		name, text string
	}{
		{"empty", ""},
		{"below_threshold", strings.Repeat("a", 127)},
		{"at_threshold", strings.Repeat("a", 128)},
		{"above_threshold", strings.Repeat("a", 129)},
		{"equal_rank_merges", strings.Repeat("a", 4097)},
		{"whitespace", strings.Repeat(" ", 4096) + "\r\n\t"},
		{"punctuation", strings.Repeat("=-_*", 1024)},
		{"long_identifier", strings.Repeat("abcdefghijklmnopqrstuvwxyz", 157)},
		{"mixed_unicode", strings.Repeat("\u4e2d\u6587\u65e5\u672c\u8a9e\U0001f642e\u0301", 128)},
		{"tool_payload", strings.Repeat(`{"type":"function_call_output","output":"alphaBeta0123456789+/=="}`+"\n", 32)},
		{"special_token_text", strings.Repeat("<|endoftext|><|fim_prefix|>", 32)},
		{"invalid_utf8", strings.Repeat("a\xff\xfe\x00b", 64)},
	}
	for _, c := range codecs {
		t.Run(c.GetName(), func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					want := originalTokenIDs(t, c, tc.text)
					ids, _, err := c.Encode(tc.text)
					require.NoError(t, err)
					assert.Equal(t, want, ids)
					count, err := c.Count(tc.text)
					require.NoError(t, err)
					assert.Equal(t, len(want), count)
				})
			}
		})
	}
}

// The unchanged v0.6.2 scan is the compatibility oracle for exact token IDs.
func originalTokenIDs(t *testing.T, c *Codec, text string) []uint {
	t.Helper()
	var ids []uint
	match, err := c.splitRegexp.FindStringMatch(text)
	require.NoError(t, err)
	for match != nil {
		piece := match.String()
		if id, ok := c.vocabulary[piece]; ok {
			ids = append(ids, id)
		} else {
			parts := c.mergePairsScan(piece)
			for i := 0; i+1 < len(parts); i++ {
				ids = append(ids, c.vocabulary[piece[parts[i].offset:parts[i+1].offset]])
			}
		}
		match, err = c.splitRegexp.FindNextMatch(match)
		require.NoError(t, err)
	}
	return ids
}

func TestLongPieceCountSharedEncoder(t *testing.T) {
	c := NewCl100kBase()
	text := strings.Repeat("a", 8192)
	want := len(originalTokenIDs(t, c, text))
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			count, err := c.Count(text)
			assert.NoError(t, err)
			assert.Equal(t, want, count)
		}()
	}
	workers.Wait()
}

func BenchmarkLongPiece(b *testing.B) {
	c := NewCl100kBase()
	for _, tc := range []struct {
		name, text string
	}{
		{"letters_8KiB", strings.Repeat("a", 8192)},
		{"letters_32KiB", strings.Repeat("a", 32768)},
		{"spaces_32KiB", strings.Repeat(" ", 32768)},
		{"identifier_32KiB", strings.Repeat("abcdefghijklmnopqrstuvwxyz", 1260)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, mode := range []string{"original", "patched"} {
				b.Run(mode, func(b *testing.B) {
					b.SetBytes(int64(len(tc.text)))
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if mode == "original" {
							c.mergePairsScan(tc.text)
						} else {
							c.mergePairs(tc.text)
						}
					}
				})
			}
		})
	}
}
