/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32001

import (
	"github.com/sas-quantome/interlace-do/internal/pkg/arranger"
	"github.com/sas-quantome/interlace-go/pkg/contractor"
	"github.com/sas-quantome/interlace-go/pkg/extractor"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

// constant program version number: to be appended to the ISO 8601 go-interlace version string.
const (
	programVersion = "2"                 // Increment this number for each release.
	primaryInpFile = "METADATA_TSV"      // Tab-delimited input file.
	assetName1     = "metadata"          // A metadata asset record identifies uniqueness.
	assetName2     = "synonym"           // A metadata identity normally contains synonyms.
	enumName       = "metadata_cv_d"     // Enum names inherit the asset name.
	hasCat         = runner.MgrFlgAliasA // Indicates that the metadata TSV contains text to hash.
	hasEnu         = runner.MgrFlgAliasB // Indicates that the metadata TSV contains CV to enumerate.
	getCID         = runner.MgrFlgAliasC // Indicates that the metadata is indexed with the CID.
	getUID         = runner.MgrFlgAliasD // Indicates that the metadata is indexed with a unique integer.
	getUUI         = runner.MgrFlgAliasE // Indicates that the metadata is indexed with UUID.
)

// Application represents the main struct for managing resources, computing, and coordinating deterministic data flows.
type Application struct {
	refiner   *contractor.Refiner   // refiner configures and handles the data refinement.
	tsv       *extractor.Extractor  // tsv to parse the metadata TSV file.
	cid       *storekeeper.CIndexer // cid indexer is needed when the first column is named suk_cid_s
	ids       []string              // ids to sort the synonym|CID|length trios.
	colNames  []string              // colNames like suk_uid_u and udk_name_s etc.
	dataTypes []byte                // dataTypes s, i, d, ... of each column
	synCols   []bool                // synCols to associate with the CID
	factor    []byte                // factor = 'E' enumerator, 'C' catalog, 'P' property
	primaryK  string                // primaryK holds the tuple's CID, UID, or UUID primary key.
	length    string                // length holds the udk_length_u value to include in the synonym|CID|length trio
	lenCol    int                   // lenCol is the column index position of udk_length_u and is > 0 because suk_cid_s
	uid       int                   // uid holds the tuple's UID integer
	records   int                   // records keep count of refined tuples.
	refining  bool                  // refining indicates the refiner needs to be closed.
}

// Start configures the Application, verifies parameters, begins logging, and returns true unless the help page is requested.
// It exits safely with an explicit error message whenever the configuration is invalid.
func (app *Application) Start() bool {
	var helpPageState bool
	app.refiner, helpPageState = contractor.NewRefiner(
		true,
		false,
		"",
		arranger.DefaultWorkflow,
		arranger.ProjectName,
		"a32001",
		"creates Metadata and Synonym assets from a TSV file.",
		programVersion,
		[]string{primaryInpFile})
	return helpPageState
}

// Commit executes the application's work.
func (app *Application) Commit() error {
	app.refiner.Mgr.Say("setting up the Metadata refiner ...")
	if commit, err := app.refiner.Commit(assetName1); err != nil {
		return err
	} else if commit == false {
		return nil
	}
	if err := app.createMetadata(); err != nil {
		return err
	}
	if len(app.ids) == 0 {
		return nil
	}
	app.refiner.Mgr.Say("setting up the Synonym refiner ...")
	app.refiner.ResetCounter()
	app.refiner.ClearBin()
	if commit, err := app.refiner.Commit(assetName2); err != nil {
		return err
	} else if commit == false {
		return nil
	}
	app.refiner.ResetCounter()
	app.refiner.ClearBin()
	return app.createSynonym()
}

// End safely exits the Application.
func (app *Application) End(err error) {
	app.refiner.End(err)
}
