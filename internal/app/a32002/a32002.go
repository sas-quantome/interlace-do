/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32002

import (
	"errors"

	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/contractor"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

const (
	// Increment this number for each release.
	programVersion = "2"

	// qryInp configuration key for target asset qry items to associate with the val items.
	qryInp = "QRY_INP_FILE"

	// keyInp configuration key for source asset key items that must match the qry items, this is the search array.
	keyInp = "KEY_INP_FILE"

	// valInp configuration key for source asset val items to insert a new binary file in the target asset.
	valInp = "VAL_INP_FILE"
)

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	refiner   *contractor.Refiner               // refiner configures and handles the data refinement.
	bin       *storekeeper.BinAsset             // bin holds a reference to the new binary file.
	binFiles  map[string]*storekeeper.Container // binFiles to read.
	ignoreVer bool                              // ignoreVer flags to ignore version verification when reading bin files.
	nDecoded  int                               // nDecoded is the number of rows in the decoded data: parallel arrays.
	nWritten  int                               // nWritten is the number of rows found in the decoded data and written.
	notFound  int                               // notFound holds the count of items without a relation found.
	nInserted int                               // nInserted holds the count of items inserted.
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	var helpPageState bool
	app.refiner, helpPageState = contractor.NewRefiner(
		true,
		true,
		"",
		"w32002",
		arranger.ProjectName,
		"a32002",
		"inserts data from one asset to another.",
		programVersion,
		[]string{qryInp, keyInp, valInp})
	return helpPageState
}

// Commit executes the application's work.
func (app *Application) Commit() error {
	app.refiner.Mgr.Say("setting up the refiner ...")
	if commit, err := app.refiner.Commit(app.refiner.Mgr.GetConfigString(runner.AssetNames)); err != nil {
		return err
	} else if commit == false {
		return nil
	}
	app.binFiles = make(map[string]*storekeeper.Container)
	if err := app.setBinInput(); err != nil {
		return err
	}
	app.setBinOutput()
	if err := app.createBin(); err != nil {
		return err
	}
	if err := app.saveBin(); err != nil {
		return err
	}
	return nil
}

// End safely exits the Application.
func (app *Application) End(err error) {
	var e []error
	if err != nil {
		e = append(e, err)
	}
	for key := range app.binFiles {
		if app.binFiles[key] != nil {
			if ret := app.binFiles[key].CloseMappedArr(); ret != nil {
				e = append(e, ret)
			}
		}
	}
	app.refiner.End(errors.Join(e...))
}
