/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32004

import (
	"flag"
	"path"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/runner"
)

// parseCommandLineArgs parses the command line arguments.
func (app *Application) parseCommandLineArgs() {
	queryTermDefault := "all the property bin files"
	// Begin by exposing the boolean flags offered to the user with default values ///////////////////////////////// 1
	help := flag.Bool(runner.ArgHelp())
	loud := flag.Bool(runner.ArgLoud())
	silent := flag.Bool(runner.ArgSilent())
	// Application-specific flags
	bit := flag.Bool("bit", false, "Unmasks the bit flags and writes the 0s and 1s using one column per flag")
	cat := flag.Bool("cat", false, "Inserts the text catalog next to the corresponding hash ID")
	enu := flag.Bool("enum", false, "Inserts the CV term next to the corresponding enum code")
	orc := flag.Bool("or", false, "OR comparison instead of AND")
	ncn := flag.Bool("ncn", false, "No column names in the output")
	// Provide the log file parameter option
	logFile := flag.String(runner.ArgLog())
	// Output controls
	flag.IntVar(&app.max, "max-rows", 0, "Maximum number of rows to write to the output, 0 means no limit (default)")
	flag.IntVar(&app.idx, "index", 0, "Index start position in the output data, 0 means the first row (default)")
	// Application-specific parameters
	var inpPath, queryTerms, outFile string
	flag.StringVar(&outFile, "out-file", "", "Output file name or the output is written to stdout")
	flag.StringVar(&inpPath, "inp-path", "./", "For example 'metadata/20260818000000' or use the default when reading a single bin file")
	flag.StringVar(&queryTerms, "query-terms", queryTermDefault, "Query term is one or more 'bin-name/operator/value' comma separated")
	// Parse the command line arguments ///////////////////////////////////////////////////////////////////////////// 2
	flag.Parse()
	// Compute the boolean flags BEFORE starting the loggers.
	app.manager.SetBitFlag(runner.MgrFlgHelp, *help)
	app.manager.SetBitFlag(runner.MgrFlgSilent, *silent)
	app.manager.SetBitFlag(runner.MgrFlgLoud, *loud)
	app.manager.SetBitFlag(bitToBool, *bit)
	app.manager.SetBitFlag(hidToText, *cat)
	app.manager.SetBitFlag(enuToTerm, *enu)
	app.manager.SetBitFlag(orComparison, *orc)
	app.manager.SetBitFlag(noColNames, *ncn)
	// Add the log file to the config BEFORE starting the loggers
	app.manager.SetConfig(runner.LogFile, *logFile, *logFile != "")
	// Start the stderr and stdout loggers for reporting accordingly to the user configuration above
	app.manager.StartLoggers(flag.NFlag())
	// The command line arguments override the existing values except for the time tag.
	app.manager.SetConfig(runner.QueryTerms, queryTerms, queryTerms != queryTermDefault)
	app.manager.SetConfig(runner.OutFile, outFile, outFile != "")
	app.manager.SetConfig(runner.InpPath, inpPath, true)
	if app.manager.GetBitFlag(runner.MgrFlgHelp) {
		return
	}
	app.ignoreVer = runner.ExtractTimeTag(inpPath) == ""
	if runner.ValidTimeTag(path.Base(inpPath)) && runner.ValidatePathExists(path.Join(inpPath, runner.PropFN)) {
		app.manager.SetConfig(runner.InpPath, path.Join(inpPath, runner.PropFN), true)
		app.manager.SetConfig(catPath, path.Join(inpPath, runner.CatFN), true)
		app.manager.SetConfig(enumPath, path.Join(inpPath, runner.EnumFN), true)
	} else {
		if queryTerms == queryTermDefault || strings.Count(queryTerms, ",") > 1 {
			app.manager.War("alternative inp-path requires a SINGLE query term")
		} else {
			app.manager.Say("inpPath:" + inpPath)
		}
	}
	if err := app.manager.CheckConfig(runner.QueryTerms, runner.OutFile, runner.InpPath); err != nil {
		app.End(err)
	}
}

// appAim returns the application's decoding and filtering instructions.
func (app *Application) appAim() string {
	return `quick lookup tool for Interlace binary files.

         The -query-terms argument order is significant and has the following syntax format: 

         'bin-file-name-1/operator-1/value-1,bin-file-name-2/operator-2/value-2, ...' 

         The output field order will match the order of the query term string. If the query term string is empty, all 
         the bin files in the Interlace Property path are decoded in sorted order where the index always leads. 

          Examples for -query-terms:

         'sdk_entrez_gene_id_u.bin,udk_pubmed_id_u.bin,udk_rif_hash_id_m.bin/eq/14990363623126830805' return genes with cited function
         'sdk_synonym_s.bin/eq/CPX-1123,udk_entrez_gene_id_u.bin,udk_protein_id_s.bin,udk_synonym_category_cv_d.bin' return synonyms
         'sdk_entrez_gene_id_u.bin/eq/2309,udk_synonym_s.bin,udk_synonym_category_cv_d.bin,udk_protein_id_s.bin' returns gene products

         'udk_nucleotide_ack_u.bin/eq/3' returns ACK nucleotide at genomic position 3
         'udk_nucleotide_ack_u.bin/eq/3+10+99' returns ACK nucleotide at genomic positions 3, 10, and 99
         'udk_nucleotide_ack_u.bin/lt/10' returns all bases before nucleotide at position 10
         'udk_nucleotide_ack_u.bin/ni/3-10' returns ACK nucleotides before 3 and after 10
         'udk_nucleotide_ack_u.bin/in/3-10' returns ACK nucleotides from position 3 to 10 inclusive
         'udk_nucleotide_ack_u.bin/in/3-10+100-200+1000-1570' returns ACK nucleotides for the three ranges

         Possible operators are:
         
         - eq: equal to
         - gt: greater than
         - lt: less than
         - ge: greater than or equal to
         - le: less than or equal to
         - ne: not equal to
         - in: query value is a substring of the field value
         - ni: is the opposite of 'in'

         When there is no value to query, the '/' delimiters are omitted, for example: 

         'sdk_score_f.bin,sdk_color_d.bin'

         To expand 'set_s' properties with multiple elements, append '/=' at the end of the bin file name: 

         'udk_name_synonyms_set_s.bin/='

         Expansion can create very large files. Only the first '/=' expansion in the query term is considered. 

         To scape the delimiter, use '\/' whenever the query value contains a '/' character.`
}
