/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32001

import (
	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/contractor"
)

// constant program version number: to be appended to the ISO 8601 go-interlace version string.
const (
	programVersion = "1"       // Increment this number for each release.
	assetName      = "synonym" // The entity's asset name.
)

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	refiner *contractor.Refiner // refiner configures and handles the data refinement.
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	var helpPageState bool
	app.refiner, helpPageState = contractor.NewRefiner(
		false,
		false,
		arranger.DnaEntityConcept,
		arranger.DefaultWorkflow,
		arranger.ProjectName,
		"a32001",
		"creates the Homo sapiens Gene entity Synonym asset from the NCBI Gene DB.",
		programVersion,
		[]string{"sdk_entrez_gene_id_u.bin", "udk_protein_id_s.bin", "udk_synonym_s.bin", "udk_synonym_category_cv_d.bin"})
	return helpPageState
}

// Commit executes the application's work.
func (app *Application) Commit() error {
	app.refiner.Mgr.Say("setting up the refiner ...")
	if commit, err := app.refiner.Commit(assetName); err != nil {
		return err
	} else if commit == false {
		return nil
	}
	app.refiner.AddBin("sdk_synonym_s.bin")             // CW tsv column 0
	app.refiner.AddBin("udk_entrez_gene_id_u.bin")      // CW tsv column 1
	app.refiner.AddBin("udk_protein_id_s.bin")          // CW tsv column 2
	app.refiner.AddBin("udk_synonym_category_cv_d.bin") // CW tsv column 3
	app.refiner.AddBin("udk_synonym_version_s.bin")     // CW tsv column 4
	app.refiner.Mgr.Say("creating the gene synonym asset ...")
	return nil
}

// End safely exits the Application.
func (app *Application) End(err error) {
	app.refiner.End(err)
}
