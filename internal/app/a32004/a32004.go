/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32004

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/cowriter"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

// constant program version number: to be appended to the ISO 8601 go-interlace version string.
const (
	programVersion = "17"                // Increment this number for each release.
	maxResults     = 1024                // Initial allocation for the result array.
	catPath        = "CAT_PATH"          // The catalog path to look for if needed.
	enumPath       = "ENUM_PATH"         // The enumerator path to look for if needed.
	hidToText      = runner.MgrFlgAliasA // To decode the hashed text.
	enuToTerm      = runner.MgrFlgAliasB // To decode the enumerated text.
	orComparison   = runner.MgrFlgAliasC // To perform OR comparison instead of the default AND.
	bitToBool      = runner.MgrFlgAliasD // To print the boolean values.
	noColNames     = runner.MgrFlgAliasE // To suppress the column names in the output.
)

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	buf       bytes.Buffer                       // buf is the buffer used to construct the putative data output.
	text      strings.Builder                    // text cache for decoded output
	manager   *runner.Manager                    // manager commits the execution flow and handles resource management.
	cw        *cowriter.Cowriter                 // cw writes and manages the decoded output.
	binFiles  []*storekeeper.Container           // binFiles to be decoded.
	enum      map[string]*storekeeper.Enumerator // enum manages the controlled vocabulary enumerators.
	term      []any                              // term holds the query terms.
	token     []string                           // token to avoid repeated allocations.
	cache     []string                           // cache is used for expanding the multi-set fields.
	bitFlags  []bool                             // bitFlags is the boolean value of the bit flags.
	expand    bool                               // expand is true if the multi-set fields are to be expanded.
	ignoreVer bool                               // ignoreVer flags to ignore version verification when reading bin files
	catTxtIdx int                                // catTxtIdx is the binFiles index of the catalog text if any
	catHidIdx int                                // catHidIdx is the binFiles index of the catalog hash if any
	binQuery  int                                // binQuery points to the SUK or SDK binFile, SUK prioritize over SDK, else first inline.
	nDecoded  int                                // nDecoded is the number of rows in the decoded data: parallel arrays.
	nWritten  int                                // nWritten is the number of rows found in the decoded data and written.
	nQFields  int                                // nQFields is the number of binFiles to perform a comparison operation on.
	idx       int                                // idx is the current index in the decoded data.
	max       int                                // max is the maximum number of rows to write to the output.
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	app.manager = runner.NewManager()
	app.manager.SetConfigTopic(arranger.ProjectName, "a32004", app.appAim(), programVersion)
	// Capture the command line arguments.
	app.parseCommandLineArgs()
	// Print the help page if requested and return false to exit the program.
	if app.manager.GetBitFlag(runner.MgrFlgHelp) == true {
		app.manager.HelpPage(runner.HelpDoc(nil, ""))
		return false
	}
	// Print the starting information and return true to commit work.
	app.manager.PrintStartingInfo()
	app.cw = cowriter.NewCowriter()
	return true
}

// Commit executes the application's work.
func (app *Application) Commit() error {
	if err := app.selectBinFiles(); err != nil {
		return err
	}
	app.findBinQuery()
	fp := app.manager.GetConfigString(runner.OutFile)
	if err := app.cw.Open(path.Dir(fp), path.Base(fp), false, false); err != nil {
		return err
	}
	err := runner.ValidateErrors(app.writeDecodedData(), app.cw.Close())
	app.manager.Say(fmt.Sprintf("total records written: %d", app.nWritten))
	return err
}

// End safely exits the Application.
func (app *Application) End(err error) {
	var e []error
	if err != nil {
		e = append(e, err)
	}
	if len(app.binFiles) > 0 {
		for i := range app.binFiles {
			app.manager.Say(fmt.Sprintf("closing bin file: %s", app.binFiles[i].Name))
			if ret := app.binFiles[i].CloseMappedArr(); ret != nil {
				e = append(e, ret)
			}
		}
	}
	app.manager.End(errors.Join(e...))
}
