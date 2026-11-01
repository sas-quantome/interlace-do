/*
 * Copyright 2026 SAS Quantome
 * SPDX-License-Identifier: Apache-2.0
 */

package a32005

import (
	"errors"
	"path"

	"github.com/sas-quantome/interlace-do/internal/pkg/pipeliner"
	"github.com/sas-quantome/interlace-go/pkg/runner"
)

// getPipeline checks the pipeline's input sources.
func (app *Application) getPipeline() error {
	if app.manager.TotalWarnings() > 0 {
		return errors.New("warnings caught before checking URI list")
	}
	if _, err := app.manager.ParseUriInpFile(app.srcNames); err != nil {
		return err
	}
	for i := range app.srcNames {
		if fp := app.manager.GetConfigString(app.srcNames[i]); fp != "" {
			if runner.ValidatePathExists(fp) == false {
				app.manager.War("input file not found: " + fp)
			}
		} else {
			app.manager.War("required URI not found: " + app.srcNames[i])
		}
	}
	if app.manager.TotalWarnings() > 0 {
		return errors.New("incomplete URI list")
	}
	return app.manager.CheckConfig(runner.MetaInpFile, runner.SopInpFile, runner.WorkInpFile)
}

// setPipeline configures the pipeline's director and creates both, the Agent and the Event workspaces.
// It also logs the pipeline's configuration details.
func (app *Application) setPipeline() error {
	if err := app.director.CreateAgentWorkspace(
		app.manager.GetConfigString(runner.AppName),
		app.manager.GetConfigString(runner.MetaInpFile),
		app.manager.GetConfigString(runner.WorkInpFile),
		app.manager.GetConfigString(runner.SopInpFile)); err != nil {
		return err
	}
	if err := app.director.CreateEventWorkspace(); err != nil {
		return err
	}
	app.director.Meta.Log(app.manager)
	return nil
}

// runPipeline handles one workflow at a time in the predefined sequence order and executes their tasks.
// For each workflow, first the Bash scripts are generated and then n-workers jobs are executed concurrently.
func (app *Application) runPipeline() error {
	fp := app.director.Base.AgentExecutionPath(app.director.AgentNS())
	workers := app.director.ConcurrentWorkers()
	concept := app.director.Meta.SearchSlotS(runner.BaseConcept)
	timeTag := app.director.Meta.SearchSlotS(runner.EventTimeTag)

	app.manager.Say(fp)

	for i := range app.director.Flux {
		workflowID := app.director.Flux[i].Work.WID
		eventNS, err := pipeliner.EventNS(concept, workflowID, timeTag)
		if err != nil {
			return err
		}

		// Insert workflow-specific state
		app.director.Meta.Insert(runner.EventActPath, app.director.Base.EventActionPath(eventNS))
		app.director.Meta.Insert(runner.EventArtPath, app.director.Base.EventArtifactPath(eventNS))
		app.director.Meta.Insert(runner.EventFunPath, app.director.Base.EventFunctionPath(eventNS))

		if err = app.director.Flux[i].CreateScripts(app.director.Meta); err != nil {
			app.clearEventPaths() // Clean up state before exiting
			return err
		}

		err = runner.ValidateErrors(
			app.director.Flux[i].RunScripts(app.director.Meta, app.manager, workers),
			app.director.Flux[i].Jobs.Save(path.Join(fp, workflowID+"_jobs.tsv")))

		if err != nil {
			app.clearEventPaths() // Clean up state before exiting
			return err
		}

		// Clean up state cleanly at the end of each workflow
		app.clearEventPaths()
	}
	return nil
}

// clearEventPaths ensures that event-specific metadata is removed to prevent cross-workflow contamination.
func (app *Application) clearEventPaths() {
	app.director.Meta.Delete(runner.EventActPath)
	app.director.Meta.Delete(runner.EventArtPath)
	app.director.Meta.Delete(runner.EventFunPath)
}
