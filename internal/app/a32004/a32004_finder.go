/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32004

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) findQueryTermsMatch() bool {
	if app.nQFields == 0 || app.binQuery < 0 {
		return true
	}
	if app.manager.GetBitFlag(orComparison) {
		if app.binFiles[app.binQuery].IsNuc == false {
			app.manager.Say("comparison search with OR enabled: slow")
		}
		return true
	}
	if app.binFiles[app.binQuery].IsNuc == false {
		app.manager.Say("comparison search with AND enabled: fast")
	}
	if app.binFiles[app.binQuery].IsSet {
		app.manager.Say(fmt.Sprintf("searching %s ...", app.binFiles[app.binQuery].Name))
		op := app.binFiles[app.binQuery].Ope
		app.binFiles[app.binQuery].Search(
			app.binFiles[app.binQuery].Term.(string), storekeeper.IN, false, false)
		app.binFiles[app.binQuery].Ope = op
	} else if app.binFiles[app.binQuery].IsNuc == false {
		app.manager.Say(fmt.Sprintf("searching %s ...", app.binFiles[app.binQuery].Name))
		app.binFiles[app.binQuery].Search(
			app.binFiles[app.binQuery].Term.(string), app.binFiles[app.binQuery].Ope, false, false)
	}
	return app.binFiles[app.binQuery].Found
}

func (app *Application) findBinFiles(fp string) []string {
	var gf []string
	entries, err := os.ReadDir(fp)
	if err != nil {
		app.End(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bin") {
			gf = append(gf, e.Name())
		}
	}
	slices.Sort(gf)
	return gf
}

func (app *Application) findBinQuery() {
	app.manager.Say(fmt.Sprintf("number of query terms: %d", app.nQFields))
	app.binQuery = -1
	if app.nQFields == 0 {
		return
	}
	marker := -1
	for i := range app.binFiles {
		if app.binFiles[i].Ope == storekeeper.IG {
			continue
		}
		if marker < 0 {
			marker = i
		}
		if app.binFiles[i].SUK {
			app.binQuery = i // sorted-unique key bin file is the priority
			break
		}
		if app.binQuery > 0 {
			continue
		}
		if app.binFiles[i].SDK {
			app.binQuery = i // first ordered SDK bin file is the secondary priority
		}
	}
	if app.binQuery < 0 {
		app.binQuery = marker // first ordered non-sorted bin file is the last priority
		if app.binFiles[app.binQuery].IsNuc {
			app.manager.Say(fmt.Sprintf("nucleotide search: fast: %s", app.binFiles[app.binQuery].Name))
		} else {
			app.manager.Say(fmt.Sprintf("linear search: slow: non-sorted key: %s", app.binFiles[app.binQuery].Name))
		}
	} else {
		app.manager.Say(fmt.Sprintf("binary search: fast: sorted key: %s", app.binFiles[app.binQuery].Name))
	}
}

func (app *Application) findCat(hid uint64) string {
	if i := runner.BinarySearchItem(app.binFiles[app.catHidIdx].MArr, hid); i >= 0 {
		return app.binFiles[app.catTxtIdx].SArr[i]
	}
	return ""
}

func (app *Application) findEnu(j int, code uint8) string {
	kw := app.binFiles[j].EnuKw
	return app.enum[kw].GetName(int(code))
}

func (app *Application) findBitFlags(x uint64) {
	if x == 0 {
		for i := range app.bitFlags {
			app.bitFlags[i] = false
		}
	}
	var k uint64 = 1
	for i := range app.bitFlags {
		app.bitFlags[i] = x&k != 0
		k = 2 * k
	}
}

func (app *Application) findContainer(entity, name string) int {
	for i := range app.binFiles {
		if app.binFiles[i].Name == name && strings.Contains(app.binFiles[i].Path, entity) {
			return i
		}
	}
	return -1
}
