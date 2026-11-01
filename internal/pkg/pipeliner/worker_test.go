/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package pipeliner

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/stretchr/testify/require"
)

func TestNewWorker(t *testing.T) {
	worker := NewWorker("w10101")
	require.NotNil(t, worker)
	require.Equal(t, "w10101", worker.WID)
}

func TestWorkflow(t *testing.T) {
	workflow := NewWorkflow("w10101")
	workflow.SetBitFlag(ActSubmit, true)
	require.True(t, workflow.GetBitFlag(ActSubmit))
}

func TestWorkflow_LoadWorkflow(t *testing.T) {
	fp := repoPath()
	workflow := NewWorkflow("w10101")
	require.NoError(t, workflow.LoadWorkflow("w10101", path.Join(fp, "test", "data", "workflow.tsv")))
	require.Equal(t, "w10101", workflow.Work.WID)
	require.NotEmpty(t, workflow.Work.Aim)
	require.NotEmpty(t, workflow.Work.Dep)
	require.NotEmpty(t, workflow.Work.Aps)
	require.Equal(t, len(workflow.Work.app), len(strings.Split(workflow.Work.Aps, " ")))
	require.NotEqual(t, 0, len(workflow.Work.dag))
	require.Equal(t, len(workflow.Work.dag), len(workflow.Work.isa))
}

func TestWorkflow_LoadSOP(t *testing.T) {
	fp := repoPath()
	workflow := NewWorkflow("w10101")
	require.NoError(t, workflow.LoadSOP("w10101", path.Join(fp, "test", "data", "sop.tsv")))
	require.Equal(t, "w10101", workflow.Work.WID)
	require.NotNil(t, workflow.Work.SOP)
}

func TestWorkflow_Order(t *testing.T) {
	fp := repoPath()
	workflow := NewWorkflow("w10101")
	require.NoError(t, workflow.LoadWorkflow("w10101", path.Join(fp, "test", "data", "workflow.tsv")))
	require.NoError(t, workflow.LoadSOP("w10101", path.Join(fp, "test", "data", "sop.tsv")))
	require.Equal(t, "w10101", workflow.Work.WID)
	require.NotNil(t, workflow.Work.SOP)
	v, err := workflow.Order("w10101")
	require.NoError(t, err)
	require.NotEqual(t, 0, len(v))
	require.NotEqual(t, -1, runner.FindItem(v, "BTC_USD_1_MIN_CSV_FILE"))
}

func TestWorkflow_JobIndices(t *testing.T) {
	workflow := NewWorkflow("w10101")
	workflow.Jobs.Insert("j01001_w10101_a10101", runner.JobStandBy)
	jix, aix, delta, msg, err := workflow.JobIndices("j01001_w10101_a10101")
	require.NoError(t, err)
	require.Equal(t, 1001, jix)
	require.Equal(t, 1, aix)
	require.Equal(t, 0, delta)
	require.Equal(t, runner.JobStandBy, msg)
	workflow.Jobs.Insert("j01002_w10101_a10101", runner.JobDone)
	_, _, delta, msg, err = workflow.JobIndices("j01002_w10101_a10101")
	require.Equal(t, 1, delta)
	require.NoError(t, err)
	require.Equal(t, runner.JobDone, msg)
}

func TestWorkflow_CreateScripts(t *testing.T) {
	meta := createMeta()
	workflow := createWorkflow(t, meta.SearchSlotS(runner.DataPath))
	require.NoError(t, workflow.CreateScripts(meta))
}

func ensureMockCSVFile(t *testing.T) {
	fp := repoPath()
	filePath := path.Join(fp, "test", "data", "btcusd_one_min_data.csv.gz")
	// Timestamps are calculated to convert exactly to:
	// - 810000.0   -> 09:00:00 UTC
	// - 1530000.0  -> 17:00:00 UTC
	// - 2070000.0  -> 23:00:00 UTC
	// They also end with 0000 to pass the b32001 filter.
	content := "Timestamp,Open,High,Low,Close,Volume\n810000.0,10.0,11.0,9.0,10.0,1.0\n1530000.0,10.0,12.0,8.0,10.0,1.0\n2070000.0,10.0,13.0,7.0,10.0,1.0\n"
	gzBytes, err := runner.StringToGzipBytes(content)
	require.NoError(t, err)
	err = os.WriteFile(filePath, gzBytes, 0644)
	require.NoError(t, err)
}

func TestWorkflow_RunScripts(t *testing.T) {
	require.NoError(t, os.Setenv("TZ", "UTC"))
	ensureMockCSVFile(t)
	mgr := runner.NewManager()
	mgr.StartLoggers(-1)
	mgr.SetBitFlag(runner.MgrFlgNoCommit, false)
	meta := createMeta()
	workflow := createWorkflow(t, meta.SearchSlotS(runner.DataPath))
	require.NoError(t, workflow.CreateScripts(meta))
	require.NoError(t, workflow.RunScripts(meta, mgr, 2))
	mgr.End(nil)
}

func createMeta() *runner.Framer {
	meta := runner.NewFramer()
	meta.Insert("BTC_USD_1_MIN_CSV_FILE", "btcusd_one_min_data.csv.gz")
	fp := repoPath()
	meta.Insert(runner.BasePath, path.Join(fp, "test", "base"))
	meta.Insert(runner.EventActPath, path.Join(fp, "test", "base", runner.EventFN, "btc", "w10101", runner.ApolloTimeTag, runner.ActFN))
	meta.Insert(runner.EventArtPath, path.Join(fp, "test", "base", runner.EventFN, "btc", "w10101", runner.ApolloTimeTag, runner.ArtFN))
	meta.Insert(runner.EventFunPath, path.Join(fp, "test", "base", runner.EventFN, "btc", "w10101", runner.ApolloTimeTag, runner.FuncFN))
	meta.Insert(runner.EnvAppsPath, path.Join(fp, "scripts", "bash"))
	meta.Insert(runner.DataPath, path.Join(fp, "test", "data"))
	return meta
}

func createWorkflow(t *testing.T, fp string) *Workflow {
	workflow := NewWorkflow("w10101")
	require.NoError(t, workflow.LoadWorkflow("w10101", path.Join(fp, "workflow.tsv")))
	require.NoError(t, workflow.LoadSOP("w10101", path.Join(fp, "sop.tsv")))
	_, err := workflow.Order("w10101")
	require.NoError(t, err)
	return workflow
}
