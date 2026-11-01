/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package pipeliner

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/sas-quantome/interlace-go/pkg/runner"
)

// defaultWorkers specifies the default number of concurrent workers.
const defaultWorkers = 4

// Director represents an abstraction that orchestrates workflows, processes, and task monitoring.
// Its components handle data storage, operational states, and configuration management.
type Director struct {
	Base *runner.Dispenser      // Base is the Agent, Entity, and Event repository dispenser.
	Task *runner.Framer         // Task tracks the agent, event, and entity tasks with their associated namespaces.
	Done *runner.Framer         // Done maps task names to their result: nil (not done), string msg, or error.
	Meta *runner.Framer         // Meta retains parameters for global configuration.
	Flux []*Workflow            // Flux handles independent workflows for execution and monitoring.
	agNS runner.BrokerNamespace // agNS represents the namespace of the agent.
	cpus int                    // cpus represents the number of CPU cores configured or available for use.
}

// NewDirector initializes and returns a new Director instance with configured dispensers, workflows, jobs, and metadata.
func NewDirector(basePath string) *Director {
	workers := min(runtime.NumCPU(), defaultWorkers)
	return &Director{
		Base: runner.NewDispenser(basePath),
		Task: runner.NewFramer(),
		Done: runner.NewFramer(),
		Meta: runner.NewFramer(),
		cpus: workers,
		Flux: []*Workflow{},
	}
}

// CreateAgentWorkspace loads the 3 input files, verifies parameters, populates the flux map of independent workflows,
// creates agent's directories, and copies the user files to the execution directory. If a configuration error is
// caught, the directories are not created and the files are not copied.
func (d *Director) CreateAgentWorkspace(agentName, metaFile, workFile, sopFile string) error {
	workflowSequence, err := d.LoadMetaData(metaFile)
	if err != nil {
		return err
	}
	if len(workflowSequence) == 0 {
		return errors.New("no workflows found in the metadata file")
	}
	timeTag := d.Meta.SearchSlotS(runner.EventTimeTag)
	if d.agNS, err = AgentNS(agentName, timeTag); err != nil {
		return err
	}
	if err = d.Base.AddAgent(agentName, timeTag); err != nil {
		return err
	}
	d.DeclareBrokerTask(agentName, d.agNS)
	var v, m []string
	for _, wid := range workflowSequence {
		workflow := NewWorkflow(wid)
		if err = workflow.LoadWorkflow(wid, workFile); err != nil {
			return err
		}
		if err = workflow.LoadSOP(wid, sopFile); err != nil {
			return err
		}
		v, err = workflow.Order(wid)
		if err != nil {
			return err
		}
		for _, key := range v {
			if key != runner.EventArtPath && key != runner.EventActPath && key != runner.EventFunPath {
				if d.Meta.SearchSlot(key) == nil {
					m = append(m, key)
				}
			}
		}
		if len(m) > 0 {
			return errors.New("missing SOT variable parameters: not declared in the metadata file: " + strings.Join(m, ", "))
		}
		d.Flux = append(d.Flux, workflow)
	}
	d.Base.BasePath = d.Meta.SearchSlotS(runner.BasePath) // validated in checkMetaData()
	if err = d.Base.CreateAgentDirectories(d.agNS); err != nil {
		return errors.New("failed to create the agent directories: " + err.Error())
	}
	d.Meta.Insert(runner.AgentExecPath, d.Base.AgentExecutionPath(d.agNS))
	d.Meta.Insert(runner.AgentConfPath, d.Base.AgentConfigurationPath())
	return d.backupAgentFiles(metaFile, workFile, sopFile)
}

// AgentNS returns the BrokerNamespace of the agent.
func (d *Director) AgentNS() runner.BrokerNamespace {
	return d.agNS
}

// CreateScripts creates event scripts for each workflow.
func (d *Director) CreateScripts() error {
	concept := d.Meta.SearchSlotS(runner.BaseConcept)
	timeTag := d.Meta.SearchSlotS(runner.EventTimeTag)
	for i := range d.Flux {
		wid := d.Flux[i].Work.WID
		ns, err := EventNS(concept, wid, timeTag)
		if err != nil {
			return err
		}
		d.Meta.Insert(runner.EventActPath, d.Base.EventActionPath(ns))
		d.Meta.Insert(runner.EventArtPath, d.Base.EventArtifactPath(ns))
		d.Meta.Insert(runner.EventFunPath, d.Base.EventFunctionPath(ns))
		if err = d.Flux[i].CreateScripts(d.Meta); err != nil {
			return err
		}
	}
	d.Meta.Delete(runner.EventActPath)
	d.Meta.Delete(runner.EventArtPath)
	d.Meta.Delete(runner.EventFunPath)
	return nil
}

// CreateEventWorkspace creates the event workspace directories for each workflow and declares the event tasks.
func (d *Director) CreateEventWorkspace() error {
	concept := d.Meta.SearchSlotS(runner.BaseConcept)
	timeTag := d.Meta.SearchSlotS(runner.EventTimeTag)
	for i := range d.Flux {
		wid := d.Flux[i].Work.WID
		ns, err := EventNS(concept, wid, timeTag)
		if err != nil {
			return err
		}
		task := TaskName(concept, wid)
		if d.GetDispenserNS(task) != "" {
			return fmt.Errorf("event name [%s] already exists", task)
		}
		if err = d.Base.AddEvent(concept, wid, timeTag); err != nil {
			return err
		}
		d.DeclareDispenserTask(task, ns)
		if err = d.Base.CreateEventDirectories(ns); err != nil {
			return errors.New("failed to create the event directories: " + err.Error())
		}
	}
	return nil
}

// CreateEntityAssets creates the entity workspace directories for each asset and declares their tasks if known.
// However, the piepleine application is designed to ignore the entity's asset names.
// The Go Interlace applications hold all the information needed to handle this internally.
// Whenever not using a Go Intelace application, this method can be used to create this setup in advance.
func (d *Director) CreateEntityAssets(assetNames *[]string) error {
	concept := d.Meta.SearchSlotS(runner.BaseConcept)
	timeTag := d.Meta.SearchSlotS(runner.EventTimeTag)
	slices.Sort(*assetNames)
	for _, asset := range *assetNames {
		ns, err := EntityNS(concept, asset, timeTag)
		if err != nil {
			return err
		}
		task := TaskName(concept, asset)
		if d.GetDispenserNS(task) != "" {
			return fmt.Errorf("entity asset name [%s] already exists", task)
		}
		if err = d.Base.AddEntity(concept, asset, timeTag); err != nil {
			return err
		}
		d.DeclareDispenserTask(task, ns)
		if err = d.Base.CreateEntityDirectories(ns); err != nil {
			return errors.New("failed to create the entity directories: " + err.Error())
		}
	}
	return nil
}

// LoadMetaData loads metadata from a file and validates its contents.
// Replaces all the {{BASE_PATH}} variables in the metadata file with the Base.BasePath value.
// DirBasePath, DirEventTimeTag, DirWorkflowIDs are required parameters.
// Returns a list of workflows IDs in execution order entered in the metadata file.
func (d *Director) LoadMetaData(mdFile string) ([]string, error) {
	if err := d.Meta.LoadFrame(mdFile); err != nil {
		return nil, err
	}
	dictionary := runner.NewFramer()
	dictionary.Insert(runner.BasePath, d.Base.BasePath)
	d.Meta.ReplaceVariables(dictionary)
	if bp := d.Meta.SearchSlot(runner.BasePath); bp != d.Base.BasePath {
		return nil, fmt.Errorf("metadata base path=[%s] but expecting=[%s]", bp, d.Base.BasePath)
	}
	if err := d.CheckMeta(runner.BasePath, runner.EventTimeTag, runner.WorkflowSeq); err != nil {
		return nil, err
	}
	if runner.ValidTimeTag(d.Meta.SearchSlotS(runner.EventTimeTag)) == false {
		return nil, errors.New("invalid time tag: " + d.Meta.SearchSlotS(runner.EventTimeTag))
	}
	if runner.ValidatePathExists(d.Meta.SearchSlotS(runner.BasePath)) == false {
		return nil, errors.New("invalid base path: " + d.Meta.SearchSlotS(runner.BasePath))
	}
	workflows := strings.Split(d.Meta.SearchSlotS(runner.WorkflowSeq), " ")
	for _, wid := range workflows {
		if runner.ValidAppID(wid) == false {
			return nil, errors.New("invalid workflow ID: " + wid)
		}
	}
	if w := d.Meta.SearchSlotS(runner.MaxWorkers); w != "" {
		if n, e := strconv.Atoi(w); e == nil {
			d.ResetCPUs(n)
		}
	} else {
		d.Meta.Insert(runner.MaxWorkers, defaultWorkers)
	}
	if d.ConcurrentWorkers() < 1 {
		d.ResetCPUs(defaultWorkers)
	}
	return workflows, nil
}

// DeclareBrokerTask registers a broker task with its namespace.
func (d *Director) DeclareBrokerTask(task string, ns runner.BrokerNamespace) {
	d.declareTask(task, string(ns))
}

// DeclareDispenserTask registers a dispenser task with its namespace.
func (d *Director) DeclareDispenserTask(task string, ns runner.DispenserNamespace) {
	d.declareTask(task, string(ns))
}

// DeleteTask removes a task from the task map.
func (d *Director) DeleteTask(task string) {
	d.Task.Delete(task)
}

// SearchTask returns the result of a task: empty msg (not done), outcome msg.
func (d *Director) SearchTask(task string) string {
	r := d.Task.SearchSlot(task)
	if r == nil {
		return ""
	}
	return r.(string)
}

// ClearAllTasks removes all tasks from the task map.
func (d *Director) ClearAllTasks() {
	d.Task.Clear()
}

// DeclareDone marks a task as completed with an optional msg or a system error, the msg can be empty but not the error.
// The msg string is ignored if the error is not nil.
func (d *Director) DeclareDone(task, msg string, err error) {
	if err != nil {
		d.Done.Insert(task, err)
	} else {
		d.Done.Insert(task, msg)
	}
}

// DeleteDone removes a task from the done map.
func (d *Director) DeleteDone(task string) {
	d.Done.Delete(task)
}

// ClearAllDone removes all tasks from the done map.
func (d *Director) ClearAllDone() {
	d.Done.Clear()
}

// SearchDone returns the result of a task: empty msg (not done), outcome msg, or error with empty msg.
func (d *Director) SearchDone(task string) (string, error) {
	r := d.Done.SearchSlot(task)
	if r == nil {
		return "", nil
	}
	switch r.(type) {
	case error:
		return "", r.(error)
	}
	return r.(string), nil
}

// GetBrokerNS returns the BrokerNamespace associated with a task or an empty string if not found.
func (d *Director) GetBrokerNS(task string) runner.BrokerNamespace {
	r := d.Task.SearchSlotS(task)
	if r == "" {
		return ""
	}
	return runner.BrokerNamespace(r)
}

// GetDispenserNS returns the DispenserNamespace associated with a task or an empty string if not found.
func (d *Director) GetDispenserNS(task string) runner.DispenserNamespace {
	r := d.Task.SearchSlotS(task)
	if r == "" {
		return ""
	}
	return runner.DispenserNamespace(r)
}

// ResetCPUs sets the number of ConcurrentWorkers equal or less than the available.
// If n = 0 sets the number of ConcurrentWorkers available.
func (d *Director) ResetCPUs(n int) {
	total := runtime.NumCPU()
	if n > 0 && n <= total {
		d.cpus = n
	} else if defaultWorkers <= total {
		d.cpus = defaultWorkers
	} else {
		d.cpus = 1
	}
}

// ConcurrentWorkers return the number of ConcurrentWorkers available.
func (d *Director) ConcurrentWorkers() int {
	return d.cpus
}

// CheckMeta checks if the required metadata parameters are present in the metadata file.
func (d *Director) CheckMeta(key ...string) error {
	var err []error
	for _, k := range key {
		if d.Meta.SearchSlot(k) == nil {
			err = append(err, errors.New("missing meta parameter for: "+k))
		}
	}
	if len(err) > 0 {
		return errors.Join(err...)
	}
	return nil
}

// TaskName generates a unique task name by combining a concept and a term (asset or workflowID) with a colon separator.
func TaskName(concept, term string) string {
	return fmt.Sprintf("%s:%s", concept, term)
}

// AgentNS returns BrokerNamespace, if an error is caught, returns an empty BrokerNamespace string.
func AgentNS(agentName, timeTag string) (runner.BrokerNamespace, error) {
	return runner.ManageNamespace(agentName, timeTag)
}

// EntityNS returns DispenserNamespace, if an error is caught, returns an empty DispenserNamespace string.
func EntityNS(entityConceptName, assetIdentityName, timeTag string) (runner.DispenserNamespace, error) {
	return runner.DispenseNamespace(entityConceptName, assetIdentityName, timeTag, false)
}

// EventNS returns DispenserNamespace, if an error is caught, returns an empty DispenserNamespace string.
// Note: the workflowID must be a ValidAppID().
func EventNS(entityConceptName, workflowID, timeTag string) (runner.DispenserNamespace, error) {
	return runner.DispenseNamespace(entityConceptName, workflowID, timeTag, true)
}

// FindMaxHours returns the maximum hour value associated with the given appID using the provided Framer instance.
// If no value is found, it returns 0.
func FindMaxHours(meta *runner.Framer, appID string) int {
	r := meta.SearchSlot(fmt.Sprintf("%s_MAX_HOURS", appID))
	if r == nil {
		return 0
	}
	return r.(int)
}

// CopyLogFile returns a log full-path to a specific agent namespace directory if conditions are met.
func (d *Director) CopyLogFile(agentNS runner.BrokerNamespace, appName, logFp string, help, logOK bool) string {
	if !help && logOK && d.Meta.Records() > 0 && agentNS != "" && logFp != "" {
		log := fmt.Sprintf("%s_%s.log", d.Meta.SearchSlotS(runner.EventTimeTag), appName)
		dst := path.Join(d.Base.AgentExecutionPath(agentNS), log)
		if logFp != dst && !runner.ValidatePathExists(dst) {
			return dst
		}
	}
	return ""
}

// backupAgentFiles copies metadata, workflow, and SOT files to the agent's execution directory structure.
func (d *Director) backupAgentFiles(metaFile, workFile, sopFile string) error {
	d.Base.AddExecutionFile(d.agNS, "meta.tsv")
	d.Base.AddExecutionFile(d.agNS, "work.tsv")
	d.Base.AddExecutionFile(d.agNS, "sot.tsv")
	// Create paths dynamically rather than relying on slice indexing
	execPath := d.Base.AgentExecutionPath(d.agNS)
	if err := runner.CopyFile(metaFile, filepath.Join(execPath, "meta.tsv")); err != nil {
		return err
	}
	if err := runner.CopyFile(workFile, filepath.Join(execPath, "work.tsv")); err != nil {
		return err
	}
	return runner.CopyFile(sopFile, filepath.Join(execPath, "sot.tsv"))
}

// declareTask registers a task with its namespace.
func (d *Director) declareTask(task string, ns string) {
	d.Task.Insert(task, ns)
}
