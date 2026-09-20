/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package seeker

import (
	"cmp"
	"math"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func MatchAny[T cmp.Ordered](op storekeeper.OperatorType, val, term T) bool {
	switch op {
	case storekeeper.EQ:
		return val == term
	case storekeeper.NE:
		return val != term
	case storekeeper.GT:
		return val > term
	case storekeeper.LT:
		return val < term
	case storekeeper.GE:
		return val >= term
	case storekeeper.LE:
		return val <= term
	case storekeeper.IN, storekeeper.NI:
		if vStr, ok := any(val).(string); ok {
			tStr := any(term).(string)
			contains := strings.Contains(vStr, tStr)
			if op == storekeeper.IN {
				return contains
			}
			return !contains
		}
		return false
	default:
		return false
	}
}

func MatchFloat(op storekeeper.OperatorType, val, term float64) bool {
	switch op {
	case storekeeper.EQ:
		return math.Abs(val-term) < storekeeper.Epsilon
	case storekeeper.NE:
		return math.Abs(val-term) >= storekeeper.Epsilon
	case storekeeper.GT:
		return val > term
	case storekeeper.LT:
		return val < term
	case storekeeper.GE:
		return val >= term
	case storekeeper.LE:
		return val <= term
	default:
		return false
	}
}

func MatchString(op storekeeper.OperatorType, val, term string) bool {
	switch op {
	case storekeeper.EQ:
		return val == term
	case storekeeper.NE:
		return val != term
	case storekeeper.GT:
		return val > term
	case storekeeper.LT:
		return val < term
	case storekeeper.GE:
		return val >= term
	case storekeeper.LE:
		return val <= term
	case storekeeper.IN:
		return strings.Contains(val, term)
	case storekeeper.NI:
		return !strings.Contains(val, term)
	default:
		return false
	}
}
