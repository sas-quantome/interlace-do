/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package a32002

import (
	"errors"
	"path"

	"github.com/sas-quantome/interlace-go/pkg/storekeeper"
)

func (app *Application) setBinInput() error {
	targetPath := path.Dir(app.manager.GetConfigString(qryInp))
	sourcePath := path.Dir(app.manager.GetConfigString(keyInp))
	if sourcePath == targetPath {
		return errors.New("key and qry bin arrays must be from different assets")
	}
	if sourcePath != path.Dir(app.manager.GetConfigString(valInp)) {
		return errors.New("key and value bin arrays must be from the same asset")
	}
	if err := app.addContainer(keyInp); err != nil {
		return err
	}
	if err := app.addContainer(valInp); err != nil {
		return err
	}
	if err := app.addContainer(qryInp); err != nil {
		return err
	}
	if app.binFiles[keyInp].DataType != app.binFiles[qryInp].DataType {
		return errors.New("key and qry bin files have different data types")
	}
	if app.binFiles[keyInp].Len != app.binFiles[valInp].Len {
		return errors.New("key and value bin arrays have different length")
	}
	if err := app.getBinFile(sourcePath, keyInp); err != nil {
		return err
	}
	if err := app.getBinFile(sourcePath, valInp); err != nil {
		return err
	}
	if err := app.getBinFile(targetPath, qryInp); err != nil {
		return err
	}
	if len(app.binFiles) != 3 {
		return errors.New("expected 3 binary files")
	}
	return nil
}

func (app *Application) getBinFile(fp, configKey string) error {
	app.binFiles[configKey].GetDataFromFile(fp, app.ignoreVer)
	return app.binFiles[configKey].Err
}

func (app *Application) addContainer(configKey string) error {
	fp := app.manager.GetConfigString(configKey)
	fn := path.Base(fp)
	app.binFiles[configKey] = app.newContainer(fp, fn)
	if app.binFiles[configKey].Err != nil {
		return app.binFiles[configKey].Err
	}
	app.binFiles[configKey].UpdateFlags(fn)
	return nil
}

func (app *Application) newContainer(dirPath, binFileName string) *storekeeper.Container {
	binFileFullPath := path.Join(dirPath, binFileName)
	container := storekeeper.NewContainer(binFileFullPath)
	app.manager.Say("reading bin file: " + binFileFullPath)
	container.GetDataFromFile(dirPath, app.ignoreVer)
	return container
}
