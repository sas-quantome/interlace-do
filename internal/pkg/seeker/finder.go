/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package seeker

import (
	"path"
	"strings"

	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

type QueryParams struct {
	Terms  []QueryTerms
	Param  map[string]any
	CatId  []uint64
	HidKey string
	TxtKey string
}

type QueryTerms struct {
	Op        storekeeper.OperatorType
	AssetName string
	BinName   string
	Term      any
	Arr       []any
}

// NewQueryParams initializes and returns a pointer to a new QueryParams instance with empty terms and parameter map.
func NewQueryParams() *QueryParams {
	q := new(QueryParams)
	q.Terms = make([]QueryTerms, 0)
	q.Param = make(map[string]any)
	return q
}

// AddQuery appends a new query term to the QueryParams based on the provided operator, asset name, bin name, and term.
func (q *QueryParams) AddQuery(op storekeeper.OperatorType, assetName, binName string, term any) {
	q.Terms = append(q.Terms, QueryTerms{Op: op, AssetName: assetName, BinName: binName, Term: term})
}

// AddParam appends a new query parameter to the QueryParams based on the provided key and value.
func (q *QueryParams) AddParam(key string, value any) {
	q.Param[key] = value
}

// Parameter retrieves the value associated with the provided key from the query parameter map.
func (q *QueryParams) Parameter(key string) any {
	if v, exists := q.Param[key]; exists {
		return v
	}
	return nil
}

// CursorOffset retrieves the cursor offset value from the query parameter map.
func (q *QueryParams) CursorOffset() int {
	if v := q.Parameter(arranger.CursorOffset); v != nil {
		return v.(int)
	}
	return 0
}

// MaxRows retrieves the maximum number of rows requested by the client.
func (q *QueryParams) MaxRows() int {
	if v := q.Parameter(arranger.MaxRows); v != nil {
		return v.(int)
	}
	return 0
}

// IsConditionalOR checks if the query parameter indicates a conditional OR operation, default is AND (false).
func (q *QueryParams) IsConditionalOR() bool {
	if v := q.Parameter(arranger.IsConditionalOR); v != nil {
		return true
	}
	return false
}

// BinFileKey returns the binary file key for the specified data array.
func (q *QueryParams) BinFileKey(i int) string {
	if i < len(q.Terms) {
		return path.Join(q.Terms[i].AssetName, q.Terms[i].BinName)
	}
	return ""
}

// PropertyToEnumerationName converts a property name to its corresponding enumeration name.
func PropertyToEnumerationName(name string) string {
	return strings.Replace(strings.Replace(name, "udk", "uuk", 1), "d.bin", "s.bin", 1)
}
