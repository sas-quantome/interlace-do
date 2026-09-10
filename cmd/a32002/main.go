/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import "github.com/sas-quantome/interlace-do/internal/app/a32002"

// main configures the application, commits work, and exits cleanly.
func main() {
	var err error
	app := new(a32002.Application)
	if app.Start() {
		err = app.Commit()
	}
	app.End(err)
}
