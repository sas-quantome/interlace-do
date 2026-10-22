/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32002

import (
	"fmt"
	"math"
	"path"

	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) saveBin() error {
	var err error
	fp := path.Join(app.binFiles[qryInp].Path, app.binFiles[valInp].Name)
	co := storekeeper.NewContainer(fp)
	switch app.bin.DataType {
	case storekeeper.SDT:
		err = co.SaveStrArr(fp, app.bin.StackSDT.Items)
	case storekeeper.IDT:
		err = co.SaveNumArr(fp, app.bin.StackIDT.Items)
	case storekeeper.NDT:
		err = co.SaveNumArr(fp, app.bin.StackNDT.Items)
	case storekeeper.DDT:
		err = co.SaveNumArr(fp, app.bin.StackDDT.Items)
	case storekeeper.UDT:
		err = co.SaveNumArr(fp, app.bin.StackUDT.Items)
	case storekeeper.MDT:
		err = co.SaveNumArr(fp, app.bin.StackMDT.Items)
	case storekeeper.FDT:
		err = co.SaveNumArr(fp, app.bin.StackFDT.Items)
	}
	app.refiner.Mgr.Say(fmt.Sprintf("total source asset items searched in %s: %d", app.binFiles[keyInp].Name, app.binFiles[keyInp].Len))
	app.refiner.Mgr.Say(fmt.Sprintf("total source asset items searched in %s: %d", app.binFiles[valInp].Name, app.binFiles[valInp].Len))
	app.refiner.Mgr.Say(fmt.Sprintf("total target asset items searched in %s: %d", app.binFiles[qryInp].Name, app.binFiles[qryInp].Len))
	app.refiner.Mgr.Say(fmt.Sprintf("total target asset items inserted in %s: %d", co.Name, app.nInserted))
	app.refiner.Mgr.Say(fmt.Sprintf("total target asset items inserted in %s for search not found: %d", co.Name, app.notFound))
	return err
}

func (app *Application) createBin() error {
	for i := range app.binFiles[qryInp].Len {
		if err := app.search(i); err != nil {
			return err
		}
		x := -1
		if app.binFiles[keyInp].Found {
			x = app.binFiles[keyInp].Index[0]
		}
		if x == -1 {
			app.pushNotFound()
		} else {
			app.pushFound(x)
		}
	}
	return nil
}

func (app *Application) setBinOutput() {
	app.bin = new(storekeeper.BinAsset)
	app.bin.FileName = app.binFiles[valInp].Name
	app.bin.DataType = app.binFiles[valInp].DataType
	if app.binFiles[valInp].UDK {
		app.bin.IdxOrder = storekeeper.UDK
	} else if app.binFiles[valInp].SDK {
		app.bin.IdxOrder = storekeeper.SDK
	} else if app.binFiles[valInp].UUK {
		app.bin.IdxOrder = storekeeper.UUK
	} else {
		app.bin.IdxOrder = storekeeper.SUK
	}
}

func (app *Application) search(i int) error {
	switch app.binFiles[keyInp].DataType {
	case storekeeper.SDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].SArr[i], storekeeper.EQ, true, false)
	case storekeeper.IDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].IArr[i], storekeeper.EQ, true, false)
	case storekeeper.NDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].NArr[i], storekeeper.EQ, true, false)
	case storekeeper.DDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].DArr[i], storekeeper.EQ, true, false)
	case storekeeper.UDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].UArr[i], storekeeper.EQ, true, false)
	case storekeeper.MDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].MArr[i], storekeeper.EQ, true, false)
	case storekeeper.FDT:
		app.binFiles[keyInp].SearchAny(app.binFiles[qryInp].FArr[i], storekeeper.EQ, true, false)
	}
	return app.binFiles[keyInp].Err
}

func (app *Application) pushNotFound() {
	switch app.binFiles[valInp].DataType {
	case storekeeper.SDT:
		app.bin.StackSDT.Push("")
	case storekeeper.IDT:
		app.bin.StackIDT.Push(math.MinInt64)
	case storekeeper.NDT:
		app.bin.StackNDT.Push(math.MinInt32)
	case storekeeper.DDT:
		app.bin.StackDDT.Push(0)
	case storekeeper.UDT:
		app.bin.StackUDT.Push(0)
	case storekeeper.MDT:
		app.bin.StackMDT.Push(0)
	case storekeeper.FDT:
		app.bin.StackFDT.Push(math.MaxFloat64)
	}
	app.notFound++
	app.nInserted++
}

func (app *Application) pushFound(x int) {
	switch app.binFiles[valInp].DataType {
	case storekeeper.SDT:
		app.bin.StackSDT.Push(app.binFiles[valInp].SArr[x])
	case storekeeper.IDT:
		app.bin.StackIDT.Push(app.binFiles[valInp].IArr[x])
	case storekeeper.NDT:
		app.bin.StackNDT.Push(app.binFiles[valInp].NArr[x])
	case storekeeper.DDT:
		app.bin.StackDDT.Push(app.binFiles[valInp].DArr[x])
	case storekeeper.UDT:
		app.bin.StackUDT.Push(app.binFiles[valInp].UArr[x])
	case storekeeper.MDT:
		app.bin.StackMDT.Push(app.binFiles[valInp].MArr[x])
	case storekeeper.FDT:
		app.bin.StackFDT.Push(app.binFiles[valInp].FArr[x])
	}
	app.nInserted++
}
