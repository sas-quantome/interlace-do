/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32001

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"uuid"

	"github.com/sas-quantome/interlace-go/pkg/extractor"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) createMetadata() error {
	var err error
	app.refiner.Mgr.Say("creating the metadata asset ...")
	app.tsv = extractor.NewExtractor(1024 * 1024 * 8)
	if err = app.tsv.Start(app.refiner.SrcInpFile(primaryInpFile)); err != nil {
		return err
	}
	bytesRead := app.tsv.Read()
	for bytesRead > 0 {
		err = app.refinement(bytesRead)
		if err != nil {
			app.refiner.Mgr.War(fmt.Sprintf("exit parser at line: %d", app.tsv.Row()))
			break
		}
		app.tsv.CatchTruncatedColumnData()
		bytesRead = app.tsv.ReadNext()
	}
	if err == nil {
		err = app.refiner.FlushAll()
	}
	return app.endRefinement(runner.ValidateErrors(err, app.tsv.End()))
}

func (app *Application) refinement(bytesRead int) error {
	for app.tsv.Index() < bytesRead {
		b := app.tsv.Buffer[app.tsv.Index()]
		if b == '\t' || b == '\n' {
			column := app.tsv.Column() // CRITICAL: Capture column index *before* Extractor increments it
			row := app.tsv.Row()
			if b == '\t' {
				app.tsv.IndexAtEndOfColumn(column)
				if app.tsv.Row() == 0 {
					app.addColName()
				}
			} else {
				app.tsv.IndexAtEndOfLine(column)
				if app.tsv.Row() == 1 {
					app.addColName()
					if err := app.startRefiner(); err != nil {
						return err
					}
				}
			}
			if row > 0 {
				if err := app.interlace(column); err != nil {
					return err
				}
			}
		}
		app.tsv.IndexAtColumnData()
		app.tsv.IncrementIndex()
	}
	app.refiner.Mgr.Say(fmt.Sprintf("total metadata lines read: %d", app.tsv.Row()))
	app.refiner.Mgr.Say(fmt.Sprintf("total metadata table rows examined: %d", app.records))
	app.refiner.Mgr.Say(fmt.Sprintf("total metadata tuples created: %d", app.records))
	app.records = 0
	return nil
}

func (app *Application) addColName() {
	app.colNames = append(app.colNames, app.tsv.ColumnData())
}

func (app *Application) interlace(column int) error {
	if column >= len(app.colNames) {
		// tolerate additional column values outside the initialized column range
		return nil // ignoring quietly
	}
	// index if requested
	var s string
	if column == 0 && app.refiner.Mgr.GetBitFlag(getCID) {
		s = app.cid.String() // e.g. AAA
		app.primaryK = s
		if s == "ZZZ" {
			return errors.New("unique CID=[ZZZ] is already indexed")
		}
		app.cid.Next() // e.g. AAB
	} else if column == 0 && app.refiner.Mgr.GetBitFlag(getUID) {
		app.uid++
		app.primaryK = strconv.Itoa(app.uid)
		s = app.primaryK
	} else if column == 0 && app.refiner.Mgr.GetBitFlag(getUUI) {
		app.primaryK = uuid.New().String()
		s = app.primaryK
	} else {
		s = app.tsv.ColumnData()
	}
	// check every single byte is the expected data type and remove strip out non-UTF-8 strings
	s = strings.ToValidUTF8(s, "")
	switch app.dataTypes[column] {
	case storekeeper.SDT:
		break
	case storekeeper.IDT:
		if _, e := strconv.ParseInt(s, 10, 64); e != nil {
			return errors.New("invalid int64 value: " + s)
		}
	case storekeeper.NDT:
		if _, e := strconv.ParseInt(s, 10, 32); e != nil {
			return errors.New("invalid int32 value: " + s)
		}
	case storekeeper.DDT:
		if app.factor[column] == 'E' {
			if s == "" {
				s = "0"
			} else {
				s = strconv.Itoa(app.refiner.Enum.Assure(s))
			}
		} else if _, e := strconv.ParseUint(s, 10, 8); e != nil {
			return errors.New("invalid uint8 value: " + s)
		}
	case storekeeper.UDT:
		if _, e := strconv.ParseUint(s, 10, 32); e != nil {
			return errors.New("invalid uint32 value: " + s)
		}
		if app.lenCol == column && app.lenCol > 0 {
			app.length = s
		}
	case storekeeper.MDT:
		if app.factor[column] == 'C' {
			if s == "" {
				s = "0"
			} else {
				s = strconv.Itoa(int(app.refiner.Cat.HashText(s)))
			}
		} else if _, e := strconv.ParseUint(s, 10, 64); e != nil {
			return errors.New("invalid uint64 value: " + s)
		}
	case storekeeper.FDT:
		if _, e := strconv.ParseFloat(s, 64); e != nil {
			return errors.New("invalid float64 value: " + s)
		}
	}
	if app.synCols[column] && (app.refiner.Mgr.GetBitFlag(getCID) ||
		app.refiner.Mgr.GetBitFlag(getUID) || app.refiner.Mgr.GetBitFlag(getUUI)) {
		app.ids = append(app.ids, s+"\t"+app.primaryK+"\t"+app.length)
		if b, a, found := strings.Cut(s, "."); found && runner.AllAreDigits(a) {
			app.ids = append(app.ids, b+"\t"+app.primaryK+"\t"+app.length) // remove the accession version number
		}
	}
	var err error
	if column == len(app.dataTypes)-1 {
		if err = app.refiner.Writer("%s\n", s); err == nil {
			err = app.refiner.FlushSeg()
			app.length = ""
			app.primaryK = ""
			app.records++
		}
	} else {
		if err = app.refiner.Writer("%s\t", s); err == nil {
			err = app.refiner.FlushSeg()
		}
	}
	return err
}
