/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32004

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) selectBinFiles() error {
	var queryBinFiles, va []string
	var op []storekeeper.OperatorType
	var ex []bool
	fp := app.manager.GetConfigString(runner.InpPath)
	queryTerms := app.queryTerms()
	binFiles := app.findBinFiles(fp)
	if len(binFiles) == 0 {
		return fmt.Errorf("no bin files found in input path: %s", fp)
	}
	if len(queryTerms) == 0 {
		if storekeeper.IsNuc(binFiles[0]) {
			return errors.New("nucleotide search requires query terms")
		}
		// default to all bin files
		queryBinFiles = binFiles
		op = make([]storekeeper.OperatorType, len(queryBinFiles))
		va = make([]string, len(queryBinFiles))
		ex = make([]bool, len(queryBinFiles))
	} else {
		// select only the bin files that match the query terms
		gotOne := false
		for _, qt := range queryTerms {
			s := strings.Split(qt, "/")
			if runner.BinarySearchItem(binFiles, s[0]) >= 0 {
				if slices.Contains(queryBinFiles, s[0]) {
					return fmt.Errorf("duplicate query term: %s", qt)
				}
				queryBinFiles = append(queryBinFiles, s[0])
				if len(s) >= 3 {
					oc := storekeeper.OperatorCode(s[1])
					if oc == storekeeper.IG {
						return fmt.Errorf("invalid operator code: %s", qt)
					}
					app.nQFields++
					op = append(op, oc)
					va = append(va, strings.ReplaceAll(s[2], "\\/", "/"))
				} else {
					op = append(op, storekeeper.IG)
					va = append(va, "")
				}
				if !gotOne {
					if slices.Contains(s, "=") {
						ex = append(ex, true)
						gotOne = true
					}
				}
				if !gotOne {
					ex = append(ex, false)
				}
			} else {
				return fmt.Errorf("bin file not found: %s", s[0])
			}
		}
	}
	app.nDecoded = 0
	for i, binFileName := range queryBinFiles {
		container := app.newContainer(fp, binFileName)
		if container.Err != nil {
			return container.Err
		}
		container.Ope = op[i]
		container.Term = va[i]
		container.UpdateFlags(binFileName)
		if container.IsNuc {
			app.manager.Say(fmt.Sprintf("indexing %s ...", binFileName))
			container.IndexNuc()
		} else {
			if app.nDecoded == 0 {
				app.nDecoded = container.Len
				if app.max == 0 {
					app.max = app.nDecoded
				}
			}
			if app.nDecoded != container.Len {
				return fmt.Errorf("bin file length mismatch: %s", binFileName)
			}
			if container.IsSet {
				container.Expand = ex[i]
			}
			if container.IsCat && app.manager.GetBitFlag(hidToText) {
				if err := app.loadCatalog(container.Property); err != nil {
					return err
				}
			} else {
				container.IsCat = false
			}
			if container.IsEnu && (app.manager.GetBitFlag(enuToTerm) || app.manager.GetBitFlag(bitToBool)) {
				if err := app.loadEnumerator(container.EnuKw); err != nil {
					return err
				}
				if container.IsBit && app.manager.GetBitFlag(bitToBool) && app.bitFlags == nil {
					app.bitFlags = make([]bool, app.enum[container.EnuKw].Len())
					app.manager.Say(fmt.Sprintf("bit flags: %d", app.enum[container.EnuKw].Len()))
				}
			} else {
				container.IsEnu = false
			}
		}
		app.binFiles = append(app.binFiles, container)
	}
	return nil
}

func (app *Application) newContainer(dirPath, binFileName string) *storekeeper.Container {
	binFileFullPath := path.Join(dirPath, binFileName)
	container := storekeeper.NewContainer(binFileFullPath)
	app.manager.Say("reading bin file: " + binFileFullPath)
	container.GetDataFromFile(dirPath, app.ignoreVer)
	return container
}

func (app *Application) loadCatalog(property string) error {
	fp := app.manager.GetConfigString(catPath)
	files := app.findBinFiles(fp)
	n := 0
	for _, f := range files {
		if strings.Contains(f, "_hash_id_") {
			container := app.newContainer(fp, f)
			if container.Err != nil {
				return container.Err
			}
			app.binFiles = append(app.binFiles, container)
			app.catHidIdx = len(app.binFiles) - 1
			n++
		} else if strings.Contains(f, "_text_") {
			container := app.newContainer(fp, f)
			if container.Err != nil {
				return container.Err
			}
			app.binFiles = append(app.binFiles, container)
			app.catTxtIdx = len(app.binFiles) - 1
			n++
		}
	}
	if n != 2 {
		return fmt.Errorf("catalog *%s* files not found", property)
	}
	if app.binFiles[app.catTxtIdx].Len != app.binFiles[app.catHidIdx].Len {
		return fmt.Errorf("corrupted catalog: %s and %s arrays have different size",
			app.binFiles[app.catTxtIdx].Name, app.binFiles[app.catHidIdx].Name)
	}
	return nil
}

func (app *Application) loadEnumerator(enuKw string) error {
	if app.enum == nil {
		app.enum = make(map[string]*storekeeper.Enumerator)
	}
	app.enum[enuKw] = storekeeper.NewEnumerator()
	fn := fmt.Sprintf("%s_cv_s.bin", enuKw)
	app.manager.Say("reading enum: " + path.Join(app.manager.GetConfigString(enumPath), fn))
	return app.enum[enuKw].Load(path.Join(app.manager.GetConfigString(enumPath), fn))
}

func (app *Application) queryTerms() []string {
	var qt []string
	if queryTerms := app.manager.GetConfigString(runner.QueryTerms); queryTerms != "" {
		qt = strings.Split(queryTerms, ",")
	}
	return qt
}
