/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32001

import (
	"errors"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) startRefiner() error {
	for column, name := range app.colNames {
		if before, ok := strings.CutSuffix(name, "/syn"); ok {
			name = before
			app.synCols = append(app.synCols, true)
		} else {
			app.synCols = append(app.synCols, false)
		}
		if len(name) < 6 {
			return errors.New("invalid Interlace bin file name: " + name)
		}
		if !strings.HasPrefix(name, "suk_") &&
			!strings.HasPrefix(name, "sdk_") &&
			!strings.HasPrefix(name, "udk_") {
			return errors.New("invalid Interlace bin file prefix: " + name)
		}
		switch name[len(name)-2:] {
		case "_s":
			if name == "suk_cid_s" {
				if column != 0 {
					return errors.New("the suk_cid_s column must be the first one in the TSV table")
				}
				app.refiner.Mgr.SetBitFlag(getCID, true)
			} else if name == "suk_uuid_s" {
				if column != 0 {
					return errors.New("the suk_uuid_s column must be the first one in the TSV table")
				}
				app.refiner.Mgr.SetBitFlag(getUUI, true)
			}

			app.dataTypes = append(app.dataTypes, storekeeper.SDT)
			app.factor = append(app.factor, 'P')
		case "_i":
			app.dataTypes = append(app.dataTypes, storekeeper.IDT)
			app.factor = append(app.factor, 'P')
		case "_n":
			app.dataTypes = append(app.dataTypes, storekeeper.NDT)
			app.factor = append(app.factor, 'P')
		case "_d":
			if strings.HasSuffix(name, "_cv_d") {
				app.factor = append(app.factor, 'E')
				app.refiner.Mgr.SetBitFlag(hasEnu, true)
			} else {
				app.factor = append(app.factor, 'P')
			}
			app.dataTypes = append(app.dataTypes, storekeeper.DDT)
		case "_u":
			if name == "suk_uid_u" {
				if column != 0 {
					return errors.New("the suk_uid_u column must be the first one in the TSV table")
				}
				app.refiner.Mgr.SetBitFlag(getUID, true)
			} else if app.refiner.Mgr.GetBitFlag(getCID) && name == "udk_length_u" {
				app.lenCol = column
			}
			app.dataTypes = append(app.dataTypes, storekeeper.UDT)
			app.factor = append(app.factor, 'P')
		case "_m":
			if strings.Contains(name, "hash_id_m") {
				app.factor = append(app.factor, 'C')
				app.refiner.Mgr.SetBitFlag(hasCat, true)
			} else {
				app.factor = append(app.factor, 'P')
			}
			app.dataTypes = append(app.dataTypes, storekeeper.MDT)
		case "_f":
			app.dataTypes = append(app.dataTypes, storekeeper.FDT)
			app.factor = append(app.factor, 'P')
		default:
			return errors.New("invalid Interlace bin file datatype suffix: " + name)
		}
		app.refiner.AddBin(name + ".bin")
	}
	if app.refiner.Mgr.GetBitFlag(getCID) {
		app.cid = storekeeper.NewCIndexer()
	}
	if app.lenCol > 0 {
		for i := range app.synCols {
			if app.synCols[i] == true {
				if i <= app.lenCol {
					return errors.New("invalid input format: udk_length_u must be located before the */syn columns")
				}
			}
		}
	}
	return app.configRefinement()
}

func (app *Application) configRefinement() error {
	if app.refiner.Mgr.GetBitFlag(hasEnu) {
		if err := app.refiner.Enum.Create([]string{"NA"}); err != nil {
			return err
		}
	}
	var err error
	if app.refiner.Mgr.GetBitFlag(hasCat) {
		err = app.refiner.OpenCowriterWithCatalog(assetName1)
	} else {
		err = app.refiner.OpenCowriter(assetName1)
	}
	if err == nil {
		app.refining = true
	}
	return nil
}

func (app *Application) endRefinement(e error) error {
	var err error
	err = e
	if err != nil && !app.refining {
		return err
	}
	if err == nil && app.refiner.Mgr.GetBitFlag(hasEnu) {
		err = app.refiner.Enum.Save(app.refiner.EnumFp(enumName))
	}
	sortArgs := "" // no args, no sorting, CIDs & UIDs are unique and generated in sorted order
	if app.refiner.Mgr.GetBitFlag(getUUI) {
		sortArgs = "none" // sorts lexicographically, they are all unique
	}
	if err != nil {
		if app.refiner.Mgr.GetBitFlag(hasCat) {
			err = runner.ValidateErrors(app.refiner.CW.Close(), app.refiner.Cat.CloseTmpFile())
		} else {
			err = app.refiner.CW.Close() // closes without writing anything
		}
	} else if app.refiner.Mgr.GetBitFlag(hasCat) {
		err = app.refiner.CloseCowriterWithCatalog(sortArgs)
	} else {
		err = app.refiner.CloseCowriter(sortArgs)
	}
	return err
}
