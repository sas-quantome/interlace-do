/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32003

import (
	"fmt"
	"path"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/extractor"
	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) createDnaSequence() error {
	var err error
	var ok bool
	fasta := extractor.NewExtractor(8 * 1024 * 1024)
	if err = fasta.Start(app.refiner.SrcInpFile(primaryInpFile)); err != nil {
		return err
	}
	app.ignore = true // nothing yet, it updates at the end of every FASTA header
	bytesRead := fasta.Read()
	app.dna.Reset('A')
	for bytesRead > 0 {
		for err == nil && fasta.Index() < bytesRead {
			b := fasta.Buffer[fasta.Index()]
			switch b {
			case '>':
				ok = false
				err = app.saveSequence()
			case '\n':
				ok = true
				colID := fasta.Column() // CRITICAL: Capture column index *before* Extractor increments it
				fasta.IndexAtEndOfLine(colID)
				app.parseHeader(fasta.ColumnData())
			}
			if err == nil && ok && b != '\n' {
				err = app.captureNucleotide(b)
			}
			if err == nil {
				fasta.IndexAtColumnData()
				fasta.IncrementIndex()
			}
		}
		if err != nil {
			app.refiner.Mgr.War(fmt.Sprintf("quiting parsing at line: %d", fasta.Row()))
			break
		}
		fasta.CatchTruncatedColumnData()
		bytesRead = fasta.ReadNext()
	}
	if err == nil {
		err = app.saveSequence()
	}
	return runner.ValidateErrors(err, fasta.End())
}

func (app *Application) captureNucleotide(b byte) error {
	switch b {
	case 'A', 'a':
		app.encodeSequence('A')
	case 'C', 'c':
		app.encodeSequence('C')
	case 'G', 'g':
		app.encodeSequence('G')
	case 'T', 't':
		app.encodeSequence('T')
	case 'N', 'n':
		app.encodeSequence('N')
	case 'B', 'b':
		app.encodeSequence('B')
	case 'D', 'd':
		app.encodeSequence('D')
	case 'H', 'h':
		app.encodeSequence('H')
	case 'K', 'k':
		app.encodeSequence('K')
	case 'M', 'm':
		app.encodeSequence('M')
	case 'R', 'r':
		app.encodeSequence('R')
	case 'S', 's':
		app.encodeSequence('S')
	case 'V', 'v':
		app.encodeSequence('V')
	case 'W', 'w':
		app.encodeSequence('W')
	case 'Y', 'y':
		app.encodeSequence('Y')
	default:
		return fmt.Errorf("invalid nucleotide: %c", b)
	}
	return nil
}

func (app *Application) encodeSequence(na byte) {
	if na == 'N' {
		if app.dna.NSpan == 0 {
			if app.dna.BitMask() != 0 {
				app.seq.Push(app.dna.BitMask())
			}
			app.dna.Clear(false)
		}
		app.dna.NSpan++
		app.dna.UpdateMask(app.dna.NSpan)
		return
	}
	if app.dna.NSpan == 1 {
		app.dna.Reset('N')
		app.dna.Encode('N')
	} else if app.dna.NSpan > 1 {
		if app.dna.BitMask() != 0 {
			app.seq.Push(app.dna.BitMask())
		} else {
			app.refiner.Mgr.War("cannot have an empty BitMask when N's > 1")
		}
		app.dna.Reset(na)
	}
	app.dna.Encode(na)
	if app.dna.NotIndexed() {
		if app.dna.BitMask() != 0 {
			app.seq.Push(app.dna.BitMask())
		}
		app.dna.Reset(na)
		app.dna.Encode(na)
	}
}

func (app *Application) parseHeader(s string) {
	if len(s) == 0 {
		return
	}
	if s[0] == '>' {
		app.uid = ""
		app.fastaHeader(s)
	}
}

func (app *Application) fastaHeader(s string) {
	app.ignore = true
	var cat int
	if strings.Contains(s, " AS:GRCh") || strings.Contains(s, " AS:hs38d1") ||
		strings.Contains(s, " SP:Human_herpesvirus_4") {
		cat = 1
	} else if strings.Contains(s, ">lcl|") {
		cat = 2
	} else if strings.Contains(s, ">HLA-") {
		cat = 3
	} else {
		app.refiner.Mgr.Say("unknown FASTA format:" + s)
		return
	}
	switch cat {
	case 1:
		// >chr1  AC:CM000663.2  gi:568336023  LN:248956422  rl:Chromosome  M5:6aef897c3d6ff0c78aff06ac189178dd  AS:GRCh38
		t := strings.Split(s, "AC:")
		if len(t) < 2 {
			app.refiner.Mgr.Say("invalid GRC FASTA format:" + s)
			return
		}
		app.uid = strings.Split(t[1], " ")[0]
	case 2:
		// >lcl|abcdef| length=1661, from=1, to=1661
		t := strings.Split(s, "|")
		if len(t) < 3 {
			app.refiner.Mgr.Say("invalid LCL FASTA format:" + s)
			return
		}
		app.uid = t[1]
	default:
		// >HLA-DQA1*06:01:01	HLA00620 5878 bp
		t := strings.Split(s, "\t")
		if len(t) < 2 {
			app.refiner.Mgr.Say("invalid HLA FASTA format:" + s)
			return
		}
		app.uid = strings.TrimLeft(s, ">")
	}
	app.ignore = false
	app.findCID()
}

func (app *Application) findCID() {
	idx := runner.BinarySearchItem(app.syn.SArr, app.uid)
	if idx >= 0 {
		app.refiner.Mgr.Say(fmt.Sprintf("encoding cid=[%s] from sequence: %s", app.cid.SArr[idx], app.uid))
		app.uid = app.cid.SArr[idx]
		app.len = int(app.nbp.UArr[idx])
		app.ignore = false
		return
	}
	app.ignore = true
	app.refiner.Mgr.Say("sequence accession number is not found: " + app.uid)
	app.uid = ""
}

func (app *Application) saveSequence() error {
	var err error
	if app.ignore == false {
		if app.dna.BitMask() != 0 {
			app.seq.Push(app.dna.BitMask())
		}
		if app.dna.NBases() != app.len {
			return fmt.Errorf("sequence length mismatch, expected=[%d], encoded bases: %d", app.len, app.dna.NBases())
		}
		app.refiner.Mgr.Say(fmt.Sprintf("saved %s, sequence length=[%d], uint32 array length=[%d], byte reduction: %dX",
			app.uid, app.len, app.seq.Len(), app.len/(4*app.seq.Len())))
		fp := app.refiner.Base.EntityPropertyPath(app.refiner.EntityNS)
		fn := fmt.Sprintf("udk_nucleotide_%s_u.bin", strings.ToLower(app.uid)) // CID lower case in file names
		co := storekeeper.NewContainer(fn)
		err = co.SaveNumArr(path.Join(fp, fn), app.seq.Items)
	}
	app.seq.Reset()
	app.dna.Clear(true)
	return err
}
