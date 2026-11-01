/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package pipeliner

import (
	"errors"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/stretchr/testify/require"
)

func TestNewDirector(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	require.NotNil(t, d)
	require.NotNil(t, d.Base)
	require.NotNil(t, d.Task)
	require.NotNil(t, d.Done)
	require.NotNil(t, d.Meta)
	require.NotNil(t, d.Flux)
	require.NotZero(t, d.cpus)
}

func TestDirector_CreateAgentWorkFrame(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	sopFn := path.Join(fp, "test", "data", "sop.tsv")
	metaFn := path.Join(fp, "test", "data", "meta.tsv")
	workFn := path.Join(fp, "test", "data", "workflow.tsv")
	require.NoError(t, d.CreateAgentWorkspace("a10101", metaFn, workFn, sopFn))
	require.NoError(t, d.CreateEventWorkspace())
	require.NoError(t, d.CreateScripts())
}

func TestDirector_LoadMetaData(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	wid, err := d.LoadMetaData(path.Join(fp, "test", "data", "meta.tsv"))
	require.NoError(t, err)
	require.Equal(t, "w10101", wid[0])
}

func TestDirector_CreateEventWorkspace(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	_, err := d.LoadMetaData(path.Join(fp, "test", "data", "meta.tsv"))
	require.NoError(t, err)
	err = d.CreateEventWorkspace()
}

func TestDirector_CreateEntityAssets(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	_, err := d.LoadMetaData(path.Join(fp, "test", "data", "meta.tsv"))
	require.NoError(t, err)
	var assets []string
	assets = append(assets, "btc_usd", "btc_eur")
	err = d.CreateEntityAssets(&assets)
	require.NoError(t, err)
}

func TestDirector_ResetCPUs(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	initialCPUs := min(runtime.NumCPU(), 4)
	require.Equal(t, initialCPUs, d.cpus)
	n := 1
	d.ResetCPUs(n)
	require.Equal(t, n, d.cpus)
	n = 1000000000
	d.ResetCPUs(n)
	require.NotEqual(t, n, d.cpus)
	require.Equal(t, defaultWorkers, d.cpus)
	require.Equal(t, d.ConcurrentWorkers(), d.cpus)
	t.Log(d.cpus)
}

func TestDirector_CheckMeta(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	d.Meta.Insert("AAA", 123)
	d.Meta.Insert("abc", "OK")
	d.Meta.Insert("xyz", 3.14)
	require.NoError(t, d.CheckMeta("AAA", "abc", "xyz"))
	require.Error(t, d.CheckMeta(""))
	require.Error(t, d.CheckMeta("zero"))
}

func TestFindMaxHours(t *testing.T) {
	meta := runner.NewFramer()
	meta.Insert("a10101_MAX_HOURS", 4)
	require.Equal(t, 4, meta.SearchSlot("a10101_MAX_HOURS"))
	require.Equal(t, 4, FindMaxHours(meta, "a10101"))
}

func TestDirector_SearchTask(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	ns, err := EventNS("btc", "w10101", runner.ApolloTimeTag)
	require.NoError(t, err)
	task1 := TaskName("btc", "w10101")
	task2 := TaskName("btc", "w10102")
	task3 := TaskName("btc", "w10103")
	d.DeclareDispenserTask(task1, ns)
	d.DeclareDispenserTask(task2, ns)
	d.DeclareDispenserTask(task3, ns)
	require.Equal(t, string(ns), d.SearchTask(task1))
	require.Equal(t, ns, d.GetDispenserNS(task1))
	require.Equal(t, 3, d.Task.Records())
	d.DeleteTask(task2)
	require.Equal(t, 2, d.Task.Records())
	d.ClearAllTasks()
	require.Zero(t, d.Task.Records())
}

func TestDirector_SearchDone(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	taskA := TaskName("btc", "usd")
	taskB := TaskName("btc", "eur")
	d.DeclareDone(taskA, "OK", nil)
	msg, err := d.SearchDone(taskA)
	require.NoError(t, err)
	require.Equal(t, "OK", msg)
	d.DeclareDone(taskB, "OK", errors.New("an error eclipses the msg"))
	_, err = d.SearchDone(taskB)
	require.Error(t, err)
	require.Equal(t, "an error eclipses the msg", err.Error())
	require.Equal(t, 2, d.Done.Records())
	d.DeleteDone(taskB)
	require.Equal(t, 1, d.Done.Records())
	d.ClearAllDone()
	require.Zero(t, d.Done.Records())
}

func TestDirector_GetBrokerNS(t *testing.T) {
	var err error
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	d.agNS, err = AgentNS("a10101", runner.ApolloTimeTag)
	require.NoError(t, err)
	d.DeclareBrokerTask("some task", d.agNS)
	require.Equal(t, d.agNS, d.GetBrokerNS("some task"))
}

func TestDirector_GetDispenserNS(t *testing.T) {
	fp := repoPath()
	d := NewDirector(path.Join(fp, "test", "base"))
	ns, err := EventNS("btc", "w10101", runner.ApolloTimeTag)
	require.NoError(t, err)
	d.DeclareDispenserTask("some event", ns)
	require.Equal(t, ns, d.GetDispenserNS("some event"))
}

func TestAgentNS(t *testing.T) {
	ns, err := AgentNS("a10101", runner.ApolloTimeTag)
	require.NoError(t, err)
	require.Equal(t, "a10101:20260101000000", string(ns))
}

func TestEventNS(t *testing.T) {
	ns, err := EventNS("btc", "w10101", runner.ApolloTimeTag)
	require.NoError(t, err)
	require.Equal(t, "btc:w10101:20260101000000", string(ns))
}

func TestEntityNS(t *testing.T) {
	ns, err := EntityNS("btc", "usd", runner.ApolloTimeTag)
	require.NoError(t, err)
	require.Equal(t, "btc:usd:20260101000000", string(ns))
}

// TmpRunPath returns the tmp path, and it gets deleted right after the test ends.
func TmpRunPath(t *testing.T) string {
	testTmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(testTmpDir, 0755))
	if _, err := os.Stat(testTmpDir); os.IsNotExist(err) {
		require.NoError(t, err)
	} else {
		t.Logf("test ephemeral path: %s", testTmpDir)
	}
	return testTmpDir
}

func repoPath() string {
	_, filename, _, _ := runtime.Caller(0)
	fp, _, _ := strings.Cut(filename, "/internal")
	return fp
}
