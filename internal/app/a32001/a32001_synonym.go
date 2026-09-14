/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32001

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/sas-quantome/interlace-go/pkg/runner"
)

// createSynonym creates the Metadata Synonym.
func (app *Application) createSynonym() error {
	if len(app.ids) == 0 {
		app.refiner.Mgr.Say("no ids collected to create a Synonym asset")
		return nil
	}
	app.refiner.Mgr.Say("creating the metadata synonym asset ...")
	app.refiner.Mgr.Say(fmt.Sprintf("total metadata synonym identifiers collected: %d", len(app.ids)))
	var err error
	if app.refiner.Mgr.GetBitFlag(getCID) {
		err = app.synonymSID("udk_cid_s.bin")
	} else if app.refiner.Mgr.GetBitFlag(getUID) {
		err = app.synonymNID("udk_uid_u.bin")
	} else if app.refiner.Mgr.GetBitFlag(getUUI) {
		err = app.synonymSID("udk_uuid_s.bin")
	}
	return err
}

func (app *Application) synonymSID(name string) error {
	var cid, syn []string
	var sln []uint32
	var cols [3]string
	slices.Sort(app.ids)
	for _, s := range app.ids {
		if runner.FillCols(s, cols[:]) >= 3 {
			syn = append(syn, cols[0])
			cid = append(cid, cols[1])
			if app.lenCol > 0 {
				if l, e := strconv.ParseUint(cols[2], 10, 32); e != nil {
					return errors.New("failed to parse length:" + cols[2])
				} else {
					sln = append(sln, uint32(l))
				}
			}
		}
	}
	if len(syn) > 0 {
		if err := app.refiner.MakeStrArr("sdk_synonym_s.bin", syn); err != nil {
			return err
		}
		if err := app.refiner.MakeStrArr(name, cid); err != nil {
			return err
		}
		if app.lenCol > 0 && len(syn) == len(sln) {
			if err := app.refiner.MakeNumArr("udk_length_u.bin", sln); err != nil {
				return err
			}
		}
	}
	app.refiner.Mgr.Say(fmt.Sprintf("total metadata synonym tuples created: %d", len(syn)))
	return nil
}

func (app *Application) synonymNID(name string) error {
	var syn []string
	var uid []uint32
	var cols [3]string
	slices.Sort(app.ids)
	for _, s := range app.ids {
		if runner.FillCols(s, cols[:]) >= 3 {
			syn = append(syn, cols[0])
			if u, e := strconv.ParseUint(cols[1], 10, 32); e != nil {
				return errors.New("failed to parse uid:" + cols[1])
			} else {
				uid = append(uid, uint32(u))
			}
		}
	}
	if len(syn) > 0 {
		if err := app.refiner.MakeStrArr("sdk_synonym_s.bin", syn); err != nil {
			return err
		}
		if err := app.refiner.MakeNumArr(name, uid); err != nil {
			return err
		}
	}
	app.refiner.Mgr.Say(fmt.Sprintf("total metadata synonym tuples created: %d", len(syn)))
	return nil
}
