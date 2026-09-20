/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package arranger

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseRanges takes a string format like "2_5-10" and expands it to []int{2, 5, 6, 7, 8, 9, 10}
func ParseRanges(input string) ([]int, error) {
	var result []int

	// Split the main parts separated by underscores
	parts := strings.SplitSeq(input, "_")

	for part := range parts {
		if part == "" {
			continue
		}

		// Check if the part represents a range (contains a hyphen)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}

			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid start number in range: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid end number in range: %s", rangeParts[1])
			}

			// Generate all numbers in between (inclusive)
			if start <= end {
				for i := start; i <= end; i++ {
					result = append(result, i)
				}
			} else {
				// Optional: Handle reverse ranges (e.g., 10-5) if needed
				for i := start; i >= end; i-- {
					result = append(result, i)
				}
			}
		} else {
			// It's a single number
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", part)
			}
			result = append(result, num)
		}
	}
	return result, nil
}
