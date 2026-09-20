/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package seeker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinder_NewQueryParams(t *testing.T) {
	q := NewQueryParams()
	require.NotNil(t, q)
	require.NotNil(t, q.Terms)
	require.NotNil(t, q.Param)
}

func TestFinder_PropertyToEnumerationName(t *testing.T) {
	require.Equal(t, "uuk_category_cv_s.bin", PropertyToEnumerationName("udk_category_cv_d.bin"))
}
