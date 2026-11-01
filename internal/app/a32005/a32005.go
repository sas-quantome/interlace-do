/*
 * Copyright 2026 SAS Quantome
 * SPDX-License-Identifier: Apache-2.0
 */

package a32005

import (
	"github.com/sas-quantome/interlace-do/internal/pkg/pipeliner"
	"github.com/sas-quantome/interlace-go/pkg/runner"
)

// constant program version number: to be appended to the ISO 8601 go-interlace version string.
const programVersion = "10" // Increment this number for each release.

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	director *pipeliner.Director // director coordinates the pipeline workflows.
	manager  *runner.Manager     // manager commits the execution flow and handles resource management.
	srcNames []string            // srcNames are constant key names to identify the three input files required.
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	app.srcNames = []string{runner.MetaInpFile, runner.SopInpFile, runner.WorkInpFile}
	app.manager = runner.NewManager()
	// Configure a pipeline manager.
	app.manager.SetBitFlag(runner.MgrFlgPipeline, true)
	// Capture the environment variables e.g., ENV_AEE_BASE_PATH, ENV_ALT_TEMP_PATH, and ENV_ALT_APPS_PATH.
	app.manager.SetConfigTopic("go-interlace", "a32005", "configures, runs, and monitors jobs and workflows in a pipeline.", programVersion)
	// Capture the command line arguments.
	app.manager.ConfigureWork("", "", true, false)
	// Print the help page if requested and return false to exit the program.
	if app.manager.GetBitFlag(runner.MgrFlgHelp) == true {
		app.manager.HelpPage(runner.HelpDoc(app.srcNames, ""))
		return false
	}
	// Print the starting information and return true to commit work.
	app.manager.PrintStartingInfo()
	// Create the pipeline director.
	app.manager.Say("configuring pipeline ...")
	app.director = pipeliner.NewDirector(app.manager.GetConfigString(runner.BasePath))
	app.director.Meta.Insert(runner.BasePath, app.manager.GetConfigString(runner.BasePath))
	app.director.Meta.Insert(runner.DataPath, app.manager.GetConfigString(runner.DataPath))
	app.director.Meta.Insert(runner.UriInpFile, app.manager.GetConfigString(runner.UriInpFile))
	if s := app.manager.GetConfigString(runner.AppsPath); s != "" {
		app.director.Meta.Insert(runner.AppsPath, s)
	}
	return true
}

// Commit executes the application's work.
func (app *Application) Commit() error {
	app.manager.Say("checking input files ...")
	if err := app.getPipeline(); err != nil {
		return err
	}
	app.manager.Say("setting up pipeline ...")
	if err := app.setPipeline(); err != nil {
		return err
	}
	app.manager.Say("running pipeline ...")
	if err := app.runPipeline(); err != nil {
		return err
	}
	return nil
}

// End safely exits the Application, copies the logfile if requested-and-is-outside the agent execution directory.
func (app *Application) End(err error) {
	if !app.manager.GetBitFlag(runner.MgrFlgHelp) {
		fp := app.director.CopyLogFile(
			app.director.AgentNS(),
			app.manager.GetConfigString(runner.AppName),
			app.manager.GetConfigString(runner.LogFile),
			app.manager.GetBitFlag(runner.MgrFlgHelp),
			app.manager.GetBitFlag(runner.MgrFlgLogOK))
		if fp != "" {
			app.manager.CopyLogFile(fp)
		}
	}
	app.manager.End(err)
}
