package logstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForEachRMWChunk(t *testing.T) {
	t.Run("splits on the chunk boundary", func(t *testing.T) {
		entries := make([]int, chRMWBatchChunk*2+7)
		for i := range entries {
			entries[i] = i
		}

		var sizes []int
		var seen []int
		require.NoError(t, forEachRMWChunk(entries, func(chunk []int) error {
			sizes = append(sizes, len(chunk))
			seen = append(seen, chunk...)
			return nil
		}))

		assert.Equal(t, []int{chRMWBatchChunk, chRMWBatchChunk, 7}, sizes)
		assert.Equal(t, entries, seen, "every entry must be visited exactly once, in order")
	})

	t.Run("single short chunk", func(t *testing.T) {
		calls := 0
		require.NoError(t, forEachRMWChunk([]int{1, 2, 3}, func(chunk []int) error {
			calls++
			assert.Len(t, chunk, 3)
			return nil
		}))
		assert.Equal(t, 1, calls)
	})

	t.Run("empty input never calls fn", func(t *testing.T) {
		called := false
		require.NoError(t, forEachRMWChunk([]int{}, func([]int) error {
			called = true
			return nil
		}))
		assert.False(t, called)
	})

	t.Run("stops at the first error", func(t *testing.T) {
		entries := make([]int, chRMWBatchChunk*3)
		calls := 0
		err := forEachRMWChunk(entries, func([]int) error {
			calls++
			if calls == 2 {
				return fmt.Errorf("boom")
			}
			return nil
		})
		require.Error(t, err)
		assert.Equal(t, 2, calls, "later chunks must not run after a failure")
	})
}

// TestClickHouseBatchCreateSpansChunks guards the chunked lock path: a batch larger
// than chRMWBatchChunk is now written as several lock/filter/insert passes, so every
// row must still land exactly once and dedup must still hold across chunk boundaries.
func TestClickHouseBatchCreateSpansChunks(t *testing.T) {
	store := trySetupClickHouseStore(t)
	ctx := context.Background()

	const total = chRMWBatchChunk*2 + 13
	ts := time.Now().UTC().Truncate(time.Millisecond)
	entries := make([]*Log, 0, total)
	for i := range total {
		entries = append(entries, chTestLog(fmt.Sprintf("ch-chunk-%04d", i), ts))
	}

	require.NoError(t, store.BatchCreateIfNotExists(ctx, entries))

	for _, i := range []int{0, chRMWBatchChunk - 1, chRMWBatchChunk, total - 1} {
		id := fmt.Sprintf("ch-chunk-%04d", i)
		got, err := store.FindByID(ctx, id)
		require.NoErrorf(t, err, "log %s should have been inserted", id)
		require.NotNil(t, got)
	}

	// Re-inserting the same batch must be a no-op, including across chunk seams.
	require.NoError(t, store.BatchCreateIfNotExists(ctx, entries))
	for _, i := range []int{0, chRMWBatchChunk, total - 1} {
		id := fmt.Sprintf("ch-chunk-%04d", i)
		_, err := store.FindByID(ctx, id)
		require.NoErrorf(t, err, "log %s should still resolve to a single row", id)
	}
}
