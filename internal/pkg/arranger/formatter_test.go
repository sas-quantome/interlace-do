/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package arranger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatter_ParseRanges(t *testing.T) {
	a, e := ParseRanges("2_5-10")
	require.NoError(t, e)
	require.Equal(t, []int{2, 5, 6, 7, 8, 9, 10}, a)
	a, e = ParseRanges("2-4_5-10")
	require.NoError(t, e)
	require.Equal(t, []int{2, 3, 4, 5, 6, 7, 8, 9, 10}, a)
	a, e = ParseRanges("5-10_2-4")
	require.NoError(t, e)
	require.Equal(t, []int{5, 6, 7, 8, 9, 10, 2, 3, 4}, a)
	a, e = ParseRanges("5-1_2-4")
	require.NoError(t, e)
	require.Equal(t, []int{5, 4, 3, 2, 1, 2, 3, 4}, a)
}
