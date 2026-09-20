/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32004

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sas-quantome/interlace-do/internal/pkg/seeker"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) writeDecodedData() error {
	if !app.findQueryTermsMatch() {
		return app.binFiles[app.binQuery].Err
	}
	if err := app.collectQueryTerms(); err != nil {
		return err
	}
	if !app.manager.GetBitFlag(noColNames) {
		if err := app.writeColNames(); err != nil {
			return err
		}
	}
	if app.nQFields == 0 {
		return app.writeNoQuery()
	}
	return app.writeQueryResult()
}

func (app *Application) writeColNames() error {
	n := ""
	k := -1
	c := 0
	for i := range app.binFiles {
		if app.binFiles[i].Property == "" {
			continue
		}
		if app.binFiles[i].IsNuc {
			continue
		}
		if app.binFiles[i].IsBit {
			k = i
			n = app.binFiles[i].Property
		} else if app.binFiles[i].IsEnu {
			n = fmt.Sprintf("%s\t%s_cv_s", app.binFiles[i].Property, app.binFiles[i].EnuKw)
		} else if app.binFiles[i].IsCat {
			s, _, _ := strings.Cut(app.binFiles[i].Property, "_hash_")
			n = fmt.Sprintf("%s\t%s_text_s", app.binFiles[i].Property, s)
		} else {
			n = app.binFiles[i].Property
		}
		if err := app.writeToken("%s", n); err != nil {
			return err
		}
		c++
	}
	if c == 0 {
		return nil
	}
	if k >= 0 {
		for i := range app.bitFlags {
			if err := app.writeToken("%s", app.enum[app.binFiles[k].EnuKw].GetName(i)); err != nil {
				return err
			}
		}
	}
	if err := app.writeAndResetBuf(); err != nil {
		return err
	}
	return nil
}

func (app *Application) writeTuple() error {
	if app.buf.Len() > 0 {
		app.cw.Buf.Write(app.buf.Bytes())
		app.cw.Buf.WriteByte('\n')
		app.nWritten++
	}
	return app.cw.BytesToDisk(8 * 1024)
}

func (app *Application) writeLine() error {
	if app.buf.Len() > 0 {
		app.cw.Buf.Write(app.buf.Bytes())
		app.cw.Buf.WriteByte('\n')
	}
	return app.cw.BytesToDisk(8 * 1024)
}

func (app *Application) writeAndResetBuf() error {
	err := app.writeLine()
	app.buf.Reset()
	return err
}

func (app *Application) writeBitFlags() {
	for _, f := range app.bitFlags {
		app.buf.WriteByte('\t')
		switch f {
		case true:
			app.buf.WriteRune('1')
		case false:
			app.buf.WriteRune('0')
		}
	}
}

func (app *Application) completeTuple() error {
	if app.expand {
		var err error
		left, right, _ := strings.Cut(app.buf.String(), "\v")
		for i := range app.cache {
			app.buf.Reset()
			_, err = fmt.Fprintf(&app.buf, "%s\t%s\t%s", left, app.cache[i], right)
			if err == nil {
				if err = app.bitTuple(); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return app.bitTuple()
}

func (app *Application) bitTuple() error {
	if app.bitFlags != nil {
		app.writeBitFlags()
	}
	return app.writeTuple()
}

func (app *Application) writeNoQuery() error {
	var err error
	for i := app.idx; i < app.nDecoded && app.nWritten < app.max; i++ {
		for j := range app.binFiles {
			if _, err = app.match(i, j); err != nil {
				return err
			}
		}
		if err = app.completeTuple(); err != nil {
			return err
		}
		app.buf.Reset()
	}
	return app.cw.BytesToDisk(1)
}

func (app *Application) writeQueryResult() error {
	var err error
	or := app.manager.GetBitFlag(orComparison)

	// Determine loop boundaries based on logic mode
	iterateAll := or || app.nQFields == 0 || app.binQuery < 0

	for r := app.idx; r < app.nDecoded && app.nWritten < app.max; r++ {
		i := r
		// If AND mode with a prioritized query, we only check the matched indices
		if !iterateAll {
			if r >= len(app.binFiles[app.binQuery].Index) {
				break // We've processed all matches for the primary query
			}
			i = app.binFiles[app.binQuery].Index[r]
		}

		nMatches := 0
		ok := true

		for j := range app.binFiles {
			if app.binFiles[j].IsNuc {
				continue
			}
			if app.binFiles[j].Property == "" {
				continue
			}
			ok, err = app.match(i, j)
			if err != nil {
				return err
			}
			if or {
				if ok && app.mustMatch(j) {
					nMatches++
				}
			} else if !ok {
				break
			}
		}

		// Write if AND succeeded, or if OR succeeded on at least one query term
		if (!or && ok) || (or && nMatches > 0) {
			if err = app.completeTuple(); err != nil {
				return err
			}
		}
		app.buf.Reset()
	}
	if app.cw.Buf.Len() > 0 {
		if err = app.cw.BytesToDisk(1); err != nil {
			return err
		}
	}
	// at the end of all non-nucleotide queries handle the nucleotides if any
	return app.queryNucleotides()
}

func (app *Application) match(i, j int) (bool, error) {
	var err error
	ok := false
	switch app.binFiles[j].DataType {
	case 's':
		app.text.WriteString(app.binFiles[j].SArr[i])
		ok, err = app.writeS(j)
		app.text.Reset()
	case 'i':
		ok, err = app.writeX(j, app.binFiles[j].IArr[i])
	case 'n':
		ok, err = app.writeX(j, app.binFiles[j].NArr[i])
	case 'd':
		val := app.binFiles[j].DArr[i]
		ok, err = app.writeX(j, val)
		if ok && err == nil && app.binFiles[j].IsEnu {
			err = app.writeToken("%s", app.findEnu(j, val))
		}
		if ok && app.binFiles[j].IsBit && app.bitFlags != nil {
			app.findBitFlags(uint64(val))
		}
	case 'u':
		ok, err = app.writeX(j, app.binFiles[j].UArr[i])
		if ok && app.binFiles[j].IsBit && app.bitFlags != nil {
			app.findBitFlags(uint64(app.binFiles[j].UArr[i]))
		}
	case 'm':
		val := app.binFiles[j].MArr[i]
		ok, err = app.writeX(j, val)
		if ok && err == nil && app.binFiles[j].IsCat {
			err = app.writeToken("%s", app.findCat(val))
		}
		if ok && app.binFiles[j].IsBit && app.bitFlags != nil {
			app.findBitFlags(val)
		}
	case 'f':
		ok, err = app.writeF(i, j)
	}
	return ok, err
}

func (app *Application) mustMatch(j int) bool {
	return app.term[j] != nil && j != app.binQuery
}

func (app *Application) writeS(j int) (bool, error) {
	if app.binFiles[j].IsSet {
		z := strings.Count(app.text.String(), "|") + 1
		if z >= maxResults {
			// nonsense
			return false, fmt.Errorf("unexpected number of pipe-delimited items in set of %s: %d", app.binFiles[j].Name, z)
		}
		if app.cache == nil {
			app.cache = make([]string, 0, maxResults)
		}
		return app.cacheSet(j)
	}
	if app.mustMatch(j) {
		if !seeker.MatchString(app.binFiles[j].Ope, app.text.String(), app.term[j].(string)) {
			return false, nil
		}
	}
	return true, app.writeToken("%s", app.text.String())
}

func (app *Application) cacheSet(j int) (bool, error) {
	app.cache = app.cache[:0] // Reset length to 0, keep capacity
	str := app.text.String()
	filterTerm := app.term[j] != nil
	app.expand = false

	// OPTIMIZATION: Zero-allocation search replacing strings.Split with strings.Cut
	t := str

	if app.binFiles[j].IsCat || app.binFiles[j].IsEnu {
		// handle catalogued and enumerated values
		for {
			x, rest, more := strings.Cut(t, "|")
			if x != "" {
				if u, e := strconv.ParseUint(x, 10, 64); e == nil {
					ok := false
					if filterTerm {
						// cache filtered results only
						if app.binFiles[j].IsCat {
							ok = seeker.MatchAny(app.binFiles[j].Ope, u, app.term[j].(uint64))
						} else {
							ok = seeker.MatchAny(app.binFiles[j].Ope, uint8(u), app.term[j].(uint8))
						}
					} else {
						// cache all of them without filtering
						ok = true
					}
					if ok {
						r := ""
						if app.binFiles[j].IsCat {
							r = app.findCat(u)
						} else {
							r = app.findEnu(j, uint8(u))
						}
						if r != "" {
							app.cache = append(app.cache, fmt.Sprintf("%s\t%s", x, r))
						}
					}
				}
			}
			if !more {
				break
			}
			t = rest
		}
	} else if filterTerm {
		// cache filtered results only
		for {
			x, rest, more := strings.Cut(t, "|")
			if x != "" && seeker.MatchAny(app.binFiles[j].Ope, x, app.term[j].(string)) {
				app.cache = append(app.cache, x)
			}
			if !more {
				break
			}
			t = rest
		}
	} else {
		// cache all of them without filtering
		for {
			x, rest, more := strings.Cut(t, "|")
			app.cache = append(app.cache, x)
			if !more {
				break
			}
			t = rest
		}
	}
	if len(app.cache) == 0 {
		return false, nil
	}
	if len(app.cache) == 1 {
		return true, app.writeToken("%s", app.cache[0])
	}
	if app.binFiles[j].Expand {
		app.expand = true
		return true, app.writeToken("\v")
	}
	return true, app.writeToken("%s", strings.Join(app.cache, "|"))
}

func (app *Application) writeToken(format string, a ...any) error {
	var err error
	if app.buf.Len() > 0 && format != "\v" {
		_, err = fmt.Fprintf(&app.buf, "\t"+format, a...) // Fixed a -> a...
	} else {
		_, err = fmt.Fprintf(&app.buf, format, a...) // Fixed a -> a...
	}
	return err
}

func (app *Application) writeX(j int, val any) (bool, error) {
	ret := true
	if app.mustMatch(j) {
		switch v := val.(type) {
		case int64:
			ret = seeker.MatchAny(app.binFiles[j].Ope, v, app.term[j].(int64))
		case int32:
			ret = seeker.MatchAny(app.binFiles[j].Ope, v, app.term[j].(int32))
		case uint8:
			ret = seeker.MatchAny(app.binFiles[j].Ope, v, app.term[j].(uint8))
		case uint32:
			ret = seeker.MatchAny(app.binFiles[j].Ope, v, app.term[j].(uint32))
		case uint64:
			ret = seeker.MatchAny(app.binFiles[j].Ope, v, app.term[j].(uint64))
		}
	}
	if !ret {
		return false, nil
	}
	return ret, app.writeToken("%d", val)
}

func (app *Application) writeF(i, j int) (bool, error) {
	val := app.binFiles[j].FArr[i]
	if app.mustMatch(j) {
		if !seeker.MatchFloat(app.binFiles[j].Ope, val, app.term[j].(float64)) {
			return false, nil
		}
	}
	return true, app.writeToken("%f", val)
}

func (app *Application) collectQueryTerms() error {
	app.term = make([]any, len(app.binFiles))
	for j := range app.binFiles {
		if app.binFiles[j].Ope == storekeeper.IG {
			app.term[j] = nil
			continue
		}

		switch app.binFiles[j].DataType {
		case 'i':
			if n, err := strconv.ParseInt(app.binFiles[j].Term.(string), 10, 64); err == nil {
				app.term[j] = n
			} else {
				return err
			}
		case 'n':
			if n, err := strconv.ParseInt(app.binFiles[j].Term.(string), 10, 32); err == nil {
				app.term[j] = int32(n) // CRITICAL FIX: Cast to underlying type
			} else {
				return err
			}
		case 'd':
			if n, err := strconv.ParseUint(app.binFiles[j].Term.(string), 10, 8); err == nil {
				app.term[j] = uint8(n) // CRITICAL FIX: Cast to underlying type
			} else {
				return err
			}
		case 'u':
			if app.binFiles[j].IsNuc {
				app.term[j] = app.binFiles[j].Term
			} else if n, err := strconv.ParseUint(app.binFiles[j].Term.(string), 10, 32); err == nil {
				app.term[j] = uint32(n) // CRITICAL FIX: Cast to underlying type
			} else {
				return err
			}
		case 'm':
			if n, err := strconv.ParseUint(app.binFiles[j].Term.(string), 10, 64); err == nil {
				app.term[j] = n
			} else {
				return err
			}
		case 'f':
			if n, err := strconv.ParseFloat(app.binFiles[j].Term.(string), 64); err == nil {
				app.term[j] = n
			} else {
				return err
			}
		}
	}
	return nil
}
