/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32002

import (
	"errors"

	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

const (
	// Increment this number for each release.
	programVersion = "1"

	// qryInp configuration key for target asset qry items to associate with the val items.
	qryInp = "QRY_INP_FILE"

	// keyInp configuration key for source asset key items that must match the qry items, this is the search array.
	keyInp = "KEY_INP_FILE"

	// valInp configuration key for source asset val items to insert a new binary file in the target asset.
	valInp = "VAL_INP_FILE"
)

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	manager   *runner.Manager                   // manager commits the execution flow and handles resource management.
	bin       *storekeeper.BinAsset             // bin holds a reference to the new binary file.
	binFiles  map[string]*storekeeper.Container // binFiles to read.
	ignoreVer bool                              // ignoreVer flags to ignore version verification when reading bin files.
	nDecoded  int                               // nDecoded is the number of rows in the decoded data: parallel arrays.
	nWritten  int                               // nWritten is the number of rows found in the decoded data and written.
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	app.manager = runner.NewManager()
	app.manager.SetConfigTopic(arranger.ProjectName, "a32002", app.appAim(), programVersion)
	// Capture the command line arguments.
	app.parseCommandLineArgs()
	// Print the help page if requested and return false to exit the program.
	if app.manager.GetBitFlag(runner.MgrFlgHelp) == true {
		app.manager.HelpPage(runner.HelpDoc(nil, ""))
		return false
	}
	// Print the starting information and return true to commit work.
	app.manager.PrintStartingInfo()
	return true
}

// Commit executes the application's work.
func (app *Application) Commit() error {
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
	app.manager.End(errors.Join(e...))
}
