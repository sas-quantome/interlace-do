/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32002

import (
	"flag"

	"github.com/sas-quantome/interlace-go/pkg/runner"
)

// parseCommandLineArgs parses the command line arguments.
func (app *Application) parseCommandLineArgs() {
	// Begin by exposing the boolean flags offered to the user with default values ///////////////////////////////// 1
	help := flag.Bool(runner.ArgHelp())
	loud := flag.Bool(runner.ArgLoud())
	silent := flag.Bool(runner.ArgSilent())
	// Provide the log file parameter option
	logFile := flag.String(runner.ArgLog())
	// Application-specific parameters
	var keyFp, valFp, qryFp string
	flag.StringVar(&keyFp, "key-inp-file", "", "Asset source full path of bin array where to search the query term")
	flag.StringVar(&valFp, "val-inp-file", "", "Asset source full path of bin array where to get the associated term")
	flag.StringVar(&qryFp, "qry-inp-file", "", "Target asset full path of bin array where to get the query term")
	// Parse the command line arguments ///////////////////////////////////////////////////////////////////////////// 2
	flag.Parse()
	// Compute the boolean flags BEFORE starting the loggers.
	app.manager.SetBitFlag(runner.MgrFlgHelp, *help)
	app.manager.SetBitFlag(runner.MgrFlgSilent, *silent)
	app.manager.SetBitFlag(runner.MgrFlgLoud, *loud)
	// Add the log file to the config BEFORE starting the loggers
	app.manager.SetConfig(runner.LogFile, *logFile, *logFile != "")
	// Start the stderr and stdout loggers for reporting accordingly to the user configuration above
	app.manager.StartLoggers(flag.NFlag())
	// The command line arguments override the existing values except for the time tag.
	if app.manager.GetBitFlag(runner.MgrFlgHelp) {
		return
	}
	// Register the input files
	app.manager.SetConfig(keyInp, keyFp, keyFp != "")
	app.manager.SetConfig(valInp, valFp, valFp != "")
	app.manager.SetConfig(qryInp, qryFp, qryFp != "")
	// Ignore the version if the key file has no time tag in the path
	app.ignoreVer = runner.ExtractTimeTag(keyFp) == ""
	// Validate parameters and make sure the input files exist
	if err := app.manager.CheckConfig(keyInp, valInp, qryInp); err != nil {
		app.End(err)
	}
}

// appAim returns the application's decoding and filtering instructions.
func (app *Application) appAim() string {
	return `inserts a data property relation from another asset.`
}
