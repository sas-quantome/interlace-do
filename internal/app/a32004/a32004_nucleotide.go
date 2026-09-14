/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32004

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) writeNucSeq(nix, length int, cid, acn string) error {
	if nix < 0 {
		return nil
	}
	t := strings.Split(app.binFiles[nix].Term.(string), "+")
	for i := range t {
		if err := app.retrieveNucSeq(nix, length, cid, acn, t[i]); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) retrieveNucSeq(nix, length int, cid, acn, t string) error {
	if nix < 0 {
		return nil
	}
	var err error
	x, y := 0, 0
	flag := false
	switch app.binFiles[nix].Ope {
	case storekeeper.IG:
		return nil // ignores search
	case storekeeper.IN: // [x, y)
		x, y, err = queryTermToInterval(t)
		x--
	case storekeeper.EQ: // [x,y)
		y, err = queryTermToIndex(t)
		x = y - 1
	case storekeeper.GT: // [x,y)
		x, err = queryTermToIndex(t)
		y = length
	case storekeeper.LT: // [x,y)
		y, err = queryTermToIndex(t)
		y--
	case storekeeper.GE: // [x,y)
		x, err = queryTermToIndex(t)
		x--
		y = length
	case storekeeper.LE: // [x,y)
		y, err = queryTermToIndex(t)
	case storekeeper.NI: // [0,x) [y,L)
		flag = true
		x, y, err = queryTermToInterval(t)
		x--
	case storekeeper.NE: // [0,x) [y,L)
		flag = true
		x, err = queryTermToIndex(t)
		y = x
		x--
	default:
		return fmt.Errorf("invalid query term: %s", t)
	}
	if err != nil {
		return err
	}
	if x < 0 {
		return errors.New("caught nucleotide index less than zero: sequence positions start at 1")
	}
	if !flag {
		return app.decodeNucSeq(nix, x, min(y, length), length, cid, acn) // [a,b)
	}
	// [0,a) [b,L)
	if err = app.decodeNucSeq(nix, 0, x, length, cid, acn); err == nil {
		err = app.decodeNucSeq(nix, y, length, length, cid, acn)
	}
	return err
}

func (app *Application) decodeNucSeq(nix, x, y, z int, cid, acn string) error {
	var err error
	if x >= z {
		return fmt.Errorf("query position=[%d] is greater than sequence length: %d", x+1, z)
	}
	w := y - x
	if x == 0 && y == z {
		_, err = fmt.Fprintf(&app.buf, ">cid|%s|gb|%s|length=%d", cid, acn, z)
	} else {
		_, err = fmt.Fprintf(&app.buf, ">cid|%s|gb|%s|segment start=%d end=%d length=%d", cid, acn, x+1, y, w)
	}
	if err != nil {
		return err
	}
	if err = app.writeAndResetBuf(); err != nil {
		return err
	}
	app.nWritten++
	app.manager.Say(fmt.Sprintf("retrieving %s [%d,%d] ...", acn, x+1, y))
	na := storekeeper.NewNucleotideEncoder()
	c := app.binFiles[nix].SearchNucLocation(x + 1)
	j := app.binFiles[nix].Index[c] - len(na.Decode(app.binFiles[nix].UArr[c]))
	k := 0
	d := false
	for i := c; i < app.binFiles[nix].Len; i++ {
		for _, base := range na.Decode(app.binFiles[nix].UArr[i]) {
			if j >= x {
				app.buf.WriteRune(base)
				if app.buf.Len() == 100 {
					if err = app.writeAndResetBuf(); err != nil {
						return err
					}
				}
				k++
			}
			j++
			if j >= y {
				d = true
				break
			}
		}
		if d {
			break
		}
	}
	if k != w {
		return fmt.Errorf("number of nucleotides decoded=[%d] does not match the expected length: %d", k, w)
	}
	return runner.ValidateErrors(app.writeAndResetBuf(), app.cw.BytesToDisk(1))
}

func queryTermToIndex(t string) (int, error) {
	index, err := strconv.Atoi(t)
	if err != nil {
		return 0, fmt.Errorf("failed to convert query term to integer: %s", t)
	}
	return index, nil
}

func queryTermToInterval(t string) (int, int, error) {
	var y int
	s := strings.Split(t, "-")
	if len(s) != 2 {
		return -1, -1, fmt.Errorf("invalid format for query term to interval: %s", t)
	}
	x, err := strconv.Atoi(s[0])
	if err != nil {
		return -1, -1, fmt.Errorf("failed to convert query term to interval: %s", s[0])
	}
	y, err = strconv.Atoi(s[1])
	if err != nil {
		return -1, -1, fmt.Errorf("failed to convert query term to interval: %s", s[1])
	}
	return x, y, nil
}

func (app *Application) queryNucleotides() error {
	cx, gx, lx := -1, -1, -1
	for i := range app.binFiles {
		if app.binFiles[i].IsNuc {
			var cid, acn string
			var length uint32
			s := strings.Split(app.binFiles[i].Property, "_")
			if len(s) == 4 {
				cid = strings.ToUpper(s[2])
				if cx == -1 {
					if err := app.assureNucMetaInContainer("suk_cid_s.bin"); err != nil {
						return err
					}
					if err := app.assureNucMetaInContainer("udk_genbank_id_s.bin"); err != nil {
						return err
					}
					if err := app.assureNucMetaInContainer("udk_length_u.bin"); err != nil {
						return err
					}
					cx = app.findContainer("metadata", "suk_cid_s.bin")
					gx = app.findContainer("metadata", "udk_genbank_id_s.bin")
					lx = app.findContainer("metadata", "udk_length_u.bin")
					if cx == -1 || gx == -1 || lx == -1 {
						return fmt.Errorf("failed loading suk_cid_s.bin, udk_genbank_id_s.bin, or udk_length_u.bin")
					}
				}
				app.binFiles[cx].Search(cid, storekeeper.EQ, false, false)
				if app.binFiles[cx].Found == false {
					return fmt.Errorf("nucleotide CID not found in DNA Metadata: %s", cid)
				}
				acn = app.binFiles[gx].SArr[app.binFiles[cx].Index[0]]
				length = app.binFiles[lx].UArr[app.binFiles[cx].Index[0]]
			} else {
				return fmt.Errorf("invalid bin filename: %s", app.binFiles[i].Property)
			}
			if err := app.writeNucSeq(i, int(length), cid, acn); err != nil {
				return err
			}
		}
	}
	return nil
}

func (app *Application) assureNucMetaInContainer(name string) error {
	fp := app.manager.GetConfigString(runner.InpPath)
	if app.findContainer("metadata", name) == -1 {
		fp = strings.Replace(fp, "sequence", "metadata", 1)
		container := app.newContainer(fp, name)
		if container.Err != nil {
			return container.Err
		}
		app.binFiles = append(app.binFiles, container)
	}
	return nil
}
