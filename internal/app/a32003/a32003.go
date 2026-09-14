/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32003

import (
	"path"

	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/contractor"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

// constant program version number: to be appended to the ISO 8601 go-interlace version string.
const (
	programVersion = "6"                 // Increment this number for each release.
	primaryInpFile = "ASSEMBLY_SEQUENCE" // Source: GCA_000001405.15_GRCh38_full_plus_hs38d1_analysis_set.fna.gz
	dnaSequence    = "sequence"          // The asset name to be created.
)

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	refiner *contractor.Refiner            // refiner configures and handles the data refinement
	syn     *storekeeper.Container         // syn contains sdk_synonym_s.bin
	cid     *storekeeper.Container         // cid contains udk_cid_s.bin
	nbp     *storekeeper.Container         // nbp contains udk_length_u.bin
	dna     *storekeeper.NucleotideEncoder // dna encodes nucleic bases into uint32 integers.
	seq     runner.Stack[uint32]           // seq holds the encoded sequence
	uid     string                         // uid of sequence
	len     int                            // len of sequence from udk_length_u.bin
	ignore  bool                           // ignore to skip unregistered sequences or bad FASTA headers
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	var helpPageState bool
	app.syn = storekeeper.NewContainer("sdk_synonym_s.bin")
	app.cid = storekeeper.NewContainer("udk_cid_s.bin")
	app.nbp = storekeeper.NewContainer("udk_length_u.bin")
	app.refiner, helpPageState = contractor.NewRefiner(
		true,
		false,
		arranger.DnaEntityConcept,
		arranger.DefaultWorkflow,
		arranger.ProjectName,
		"a32003",
		"creates the DNA entity Sequence asset from the NCBI's reference genome.",
		programVersion,
		[]string{primaryInpFile})
	return helpPageState
}

// Commit executes the application's work.
func (app *Application) Commit() error {
	app.refiner.Mgr.Say("setting up the Metadata refiner ...")
	if commit, err := app.refiner.Commit(dnaSequence); err != nil {
		return err
	} else if commit == false {
		return nil
	}
	app.refiner.Mgr.Say("creating the DNA Sequence asset ...")
	fp := app.refiner.Base.SwapAssetPath(app.refiner.EntityNS, dnaSequence, "synonym")
	if err := app.syn.OpenStrArr(path.Join(fp, "sdk_synonym_s.bin"), false); err != nil {
		return err
	}
	if err := app.cid.OpenStrArr(path.Join(fp, "udk_cid_s.bin"), false); err != nil {
		return err
	}
	if err := app.nbp.OpenNumArr(path.Join(fp, "udk_length_u.bin"), false); err != nil {
		return err
	}
	app.dna = storekeeper.NewNucleotideEncoder()
	if err := app.createDnaSequence(); err != nil {
		return err
	}
	return nil
}

// End safely exits the Application.
func (app *Application) End(err error) {
	app.refiner.End(
		runner.ValidateErrors(
			err,
			app.syn.CloseMappedArr(),
			app.cid.CloseMappedArr(),
			app.nbp.CloseMappedArr()))
}
