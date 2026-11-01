/*
 * Copyright 2026 Quantome SAS
 * SPDX-License-Identifier: Apache-2.0
 */

package pipeliner

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sas-quantome/interlace-go/pkg/runner"
	"github.com/sas-quantome/interlace-go/pkg/tabulator"
)

// SOP code constants.
const (
	SopCodeFlag  = "F"
	SopCodeNull  = "."
	SopOptional  = "O"
	SopCodePBS   = "P"
	SopCodeReq   = "R"
	SopCodeSlurm = "S"
)

const JobIdSpan = 1000

// ActBitFlag represents a bitmask for directory-related flags, stored as a 16-bit unsigned integer.
type ActBitFlag uint16

// Constants to flag operation and activity indicators.
const (
	ActSlurm ActBitFlag = 1 << iota
	ActPBS
	ActSubmit
)

// Workflow represents a unit of work consisting of jobs with their own configuration and dependency management.
type Workflow struct {
	Jobs *runner.Framer // jobs monitors the process states: a job is the execution of a script generated at runtime.
	Work *Worker        // work handles one SOP workflow carrying at least one job.
	flag Indicator      // flag flags operation signals.
}

// Indicator encapsulates a bit mask for tracking workflow-related flags.
type Indicator struct {
	mu      sync.RWMutex
	bitFlag runner.BitMask[ActBitFlag]
}

func NewWorkflow(workflowID string) *Workflow {
	return &Workflow{
		Work: NewWorker(workflowID),
		Jobs: runner.NewFramer(),
		flag: Indicator{},
	}
}

// Constants for SOP table column indices.
const (
	SopWorkflowID = iota
	SopAppID
	SopCode
	SopSlot
	SopFiller
)

// Worker handles one workflow.
type Worker struct {
	SOP *tabulator.Tabulator
	dMu sync.Mutex // CRITICAL: Protects dag and isa from concurrent access
	WID string     // workflow ID
	Aim string     // activity filler from the workflow file
	Aps string     // workflow filler from the workflow file
	Dep string     // pipeline filler from the workflow file
	app []string   // Aps[DAG-Vertex] = application name from the workflow filler; where Vertex=0 is the runner
	dag []int      // dag[i] = DAG vertex or 0; where 0 splits dependency sequences
	isa []int      // isa[i] = vertex state = 0 (app), -1 (dep), or N (job); where N is the number of parallel jobs per app.
	err error      // err handles errors by workflow.
}

// NewWorker creates and returns a new Worker instance initialized with the associated workflowID.
func NewWorker(workflowID string) *Worker {
	return &Worker{
		SOP: tabulator.NewTabulator(),
		WID: workflowID,
	}
}

func (workflow *Workflow) RunScripts(meta *runner.Framer, mgr *runner.Manager, maxGoRoutines int) error {
	var err error
	if mgr.GetBitFlag(runner.MgrFlgNoCommit) == false {
		workflow.SetBitFlag(ActSubmit, true)
	}
	scr := NewScriptor(
		meta.SearchSlotS(runner.EventActPath),
		meta.SearchSlotS(runner.EventFunPath),
		workflow.Work.app[0]) // the runner is 0
	jobs := workflow.Jobs.Grep(runner.JobStandBy)
	for _, jobName := range jobs {
		scr.jobName = jobName
		scr.jobIndex, scr.apsIndex, scr.delta, _, err = workflow.JobIndices(jobName)
		if err != nil {
			return err
		}
	}
	nJobs := len(jobs)
	mgr.Say(fmt.Sprintf("standby jobs: %d", nJobs))
	if nJobs == 0 {
		return errors.New("no more jobs to run")
	}
	jobInp := make(chan Scriptor, nJobs)
	jobOut := make(chan Scriptor, nJobs)
	for i := range maxGoRoutines {
		go workflow.runScriptor(i, jobInp, jobOut, mgr)
	}
	for j := range jobs {
		var job Scriptor
		job.sopHead = scr.sopHead
		job.actPath = scr.actPath
		job.funPath = scr.funPath
		job.jobName = jobs[j]
		job.jobIndex, job.apsIndex, job.delta, _, err = workflow.JobIndices(jobs[j])
		if hrs := FindMaxHours(meta, workflow.Work.app[job.apsIndex]); hrs > 0 {
			job.maxHours = hrs
		}
		jobInp <- job
	}
	close(jobInp)
	var c int
	for c = 0; c < nJobs; c++ {
		<-jobOut
	}
	close(jobOut)
	mgr.Say(fmt.Sprintf("closed jobs: %d", c))
	return workflow.Work.err
}

func (workflow *Workflow) runScriptor(goRoutine int, jobInp <-chan Scriptor, jobOut chan<- Scriptor, mgr *runner.Manager) {
	for job := range jobInp {
		mgr.Say(fmt.Sprintf("worker Go routine=[%d] setting job: %s", goRoutine, job.jobName))
		for {
			if workflow.Work.err != nil {
				mgr.War(fmt.Sprintf("worker Go routine=[%d] skipped job: %s", goRoutine, job.jobName))
				workflow.Jobs.Insert(job.jobName, "Skipped")
				jobOut <- job
				break
			}
			if workflow.jobBalancer(job.jobIndex - job.delta) {
				var err error
				if workflow.GetBitFlag(ActPBS) {
					err = workflow.runPBS(job)
				} else if workflow.GetBitFlag(ActSlurm) {
					err = workflow.runSlurm(job)
				} else {
					err = workflow.runBash(job)
				}
				mgr.Say(fmt.Sprintf("worker Go routine=[%d] running job: %s", goRoutine, job.jobName))
				workflow.jobEnded(job, goRoutine, err, mgr)
				jobOut <- job
				break
			}
			// CRITICAL: Prevent CPU busy-wait spin; don't use wg.Add and wg.Done; instead do in here:
			time.Sleep(1 * time.Second)
		}
	}
}

// jobBalancer returns true if a job has no upstream task dependencies, a job is considered done at this point.
// idx 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7
// dag 1 0 2 1 0 3 2 0 4 2 0 5 4 3 0 6 5 0
// isa 0 1 0 - 1 0 - 1 0 - 1 0 - - 1 0 - 3
func (workflow *Workflow) jobBalancer(jobIndex int) bool {
	workflow.Work.dMu.Lock()
	defer workflow.Work.dMu.Unlock()

	found := false
	for i := range workflow.Work.dag {
		if found {
			if workflow.Work.isa[i] < 1 {
				break
			}
			if workflow.Work.dag[i] == 0 {
				if workflow.Work.isa[i] > 1 {
					workflow.Work.isa[i]--
					return true
				}
				if workflow.Work.isa[i] == 1 {
					return true
				}
				break
			}
		} else if workflow.Work.dag[i] == jobIndex && workflow.Work.isa[i] == 0 {
			found = true
		}
	}
	return false
}

// jobEnded updates the workflow's jobs status and ISA state after revising the job log file.
func (workflow *Workflow) jobEnded(scr Scriptor, goRoutine int, err error, mgr *runner.Manager) {
	if err == nil {
		if err = mgr.ParseJobLogFile(path.Join(scr.actPath, scr.jobName+".log")); err == nil {
			workflow.Jobs.Insert(scr.jobName, "Success")
			workflow.Work.dMu.Lock()
			for i := range workflow.Work.dag {
				if workflow.Work.dag[i] == scr.jobIndex {
					workflow.Work.isa[i] = 1 // done
				}
			}
			workflow.Work.dMu.Unlock()
			mgr.Say(fmt.Sprintf("worker Go routine=[%d] success job: %s", goRoutine, scr.jobName))
			return
		}
		mgr.War(fmt.Sprintf("worker Go routine=[%d] failure job: %s: %s", goRoutine, scr.jobName, err.Error()))
	} else {
		mgr.War(fmt.Sprintf("worker Go routine=[%d] failure job: %s: %s", goRoutine, scr.jobName, err.Error()))
	}
	workflow.Jobs.Insert(scr.jobName, "Failure")
	workflow.Work.err = err
}

// commit updates the workflow's jobs status and returns true if the workflow is configured to submit the job.
func (workflow *Workflow) commit(jobName string) bool {
	workflow.Jobs.Insert(jobName, "Started")
	return workflow.GetBitFlag(ActSubmit)
}

// runBash executes a script in single node mode and waits for termination.
func (workflow *Workflow) runBash(scr Scriptor) error {
	if workflow.commit(scr.jobName) {
		err := scr.run("bash")
		if err == nil {
			workflow.Jobs.Insert(scr.jobName, "Submitted")
		}
		return err
	}
	return nil
}

// runPBS submits a PBS script and waits for termination.
func (workflow *Workflow) runPBS(scr Scriptor) error {
	if workflow.commit(scr.jobName) {
		err := scr.waitSchedulerFinishes(scr.submit("qsub"))
		if err == nil {
			workflow.Jobs.Insert(scr.jobName, "Submitted")
		}
		return err
	}
	return nil
}

// runSlurm submits a Slurm script and waits for termination.
func (workflow *Workflow) runSlurm(scr Scriptor) error {
	if workflow.commit(scr.jobName) {
		err := scr.waitSchedulerFinishes(scr.submit("sbatch"))
		if err == nil {
			workflow.Jobs.Insert(scr.jobName, "Submitted")
		}
		return err
	}
	return nil
}

// LoadJobs loads jobs from a TSV or GOB file.
func (workflow *Workflow) LoadJobs(fn string) error {
	return workflow.Jobs.LoadFrame(fn)
}

// SetBitFlag sets the specified bit flag to the given value.
func (workflow *Workflow) SetBitFlag(flag ActBitFlag, value bool) {
	workflow.flag.mu.Lock()
	defer workflow.flag.mu.Unlock()
	if value {
		workflow.flag.bitFlag.Add(flag)
	} else {
		workflow.flag.bitFlag.Clear(flag)
	}
}

// GetBitFlag returns the value of the specified bit flag.
func (workflow *Workflow) GetBitFlag(flag ActBitFlag) bool {
	workflow.flag.mu.RLock()
	defer workflow.flag.mu.RUnlock()
	return workflow.flag.bitFlag.Has(flag)
}

/*

The workflow format consists of three columns:

1) Workflow ID: an identifier with valid format e.g., w10101
2) Slot: a controlled vocabulary term [activity|workflow|pipeline] to denote the filler data
3) Filler: the declared value

Example of a workflow file:

w00101	activity	Runs mock scripts for bash.
w00101	workflow	a01044 a01001 a01011 a01003 a01023 a1005
w00101	pipeline	1 0 2 1 0 3 2 0 4 2 0 5 4 3 0

- Activity is the workflow's aim, purpose, or goal in simple terms.
- Workflow is the list of applications to execute in proper order. The first one is the runner.
- Pipeline is an integer sequence that encodes the direct acyclic graph (DAG) representing the batch processing chain.

DAG sequences declared in a workflow are expanded whenever parallel jobs are configured in the SOP table. This
conveniently prevents the user from doing the DAG expansion by hand.

How to encode the DAG:

Let the workflow be [a01044 a01001 a01011 a01003 a01023 a1005].
Their indexes will be 0,1,2,3,4 and 5 (start always at 0 and increasing order from left to right is expected).
Index 0 is the program orchestrating the pipeline so is not part of the execution sequence.
In this example, the DAG vertices representing the sequence are 1 to 5.
With these vertices consider the pipeline DAG order [1 0 2 1 0 3 2 0 4 2 0 5 4 3 0]
The DAG sequence is made of positive integers, starts always with 1 and uses 0 as a separator or terminator.

Let's decode the DAG sequence above.

First, note that vertex 0 is the runner, has no dependencies, and 0 is used as a separator between the encoded
dependencies. To build a DAG, the dependencies must be known by the pipeline author. Here we assume the following:

Vertex 1 has no dependents, so we encode ............. 1 0 ..... 1 0
Vertex 2 depends on vertex 1, so we encode ........... 2 1 ..... 1 0 2 1 0
Vertex 3 depends on vertex 2, so we encode ........... 3 2 ..... 1 0 2 1 0 3 2 0
Vertex 4 depends on vertex 2, so we encode ........... 4 2 ..... 1 0 2 1 0 3 2 0 4 2 0
Vertex 5 depends on vertices 3 and 4, so we encode ... 5 4 3 ... 1 0 2 1 0 3 2 0 4 2 0 5 4 3 0

*/

// LoadWorkflow loads the workflow file and validates the expected structure: activity, workflow, and pipeline.
func (workflow *Workflow) LoadWorkflow(wid, fn string) error {
	scanner := runner.NewScanner(0)
	if err := scanner.Start(fn); err != nil {
		return runner.ValidateErrors(err, scanner.Close())
	}
	var err error
	n := 0
	for err == nil && scanner.Scan() && n < 4 {
		s := strings.Split(scanner.Text(), "\t")
		if len(s) == 3 {
			if s[0] == wid {
				switch s[1] {
				case "activity":
					workflow.Work.Aim = strings.TrimSpace(s[2])
					n++
				case "workflow":
					workflow.Work.WID = wid
					workflow.Work.Aps = strings.TrimSpace(s[2])
					workflow.Work.app = strings.Split(workflow.Work.Aps, " ")
					n++
				case "pipeline":
					workflow.Work.Dep = strings.TrimSpace(s[2])
					err = workflow.pipelineParser()
					n++
				}
			}
		}
	}
	if scanner.Err() != nil {
		return runner.ValidateErrors(scanner.Err(), scanner.Close())
	}
	if n != 3 || err != nil {
		return runner.ValidateErrors(err, scanner.Close())
	}
	return runner.ValidateErrors(scanner.Err(), scanner.Close())
}

// pipelineParser parses the pipeline sequence and validates the DAG sequence.
// dag 1 0 2 1 0 3 2 0 4 2 0 5 4 3 0 6 5 0
// isa 0 1 0 - 1 0 - 1 0 - 1 0 - - 1 0 - 3
func (workflow *Workflow) pipelineParser() error {
	var err error
	s := strings.Split(workflow.Work.Dep, " ")
	workflow.Work.dag = make([]int, len(s))
	workflow.Work.isa = make([]int, len(s))
	firstApp := JobIdSpan + 1
	pre := 0
	k, vertex := -1, 0
	for i, v := range s {
		if vertex, err = strconv.Atoi(v); err != nil {
			return errors.New("unexpected values in the pipeline sequence")
		}
		if vertex == 0 {
			workflow.Work.dag[i] = 0
		} else {
			workflow.Work.dag[i] = dag(vertex)
		}
		if i == 0 && workflow.Work.dag[i] != firstApp {
			return errors.New("pipeline sequence must start with 1")
		}
		if pre == 0 {
			workflow.Work.isa[i] = 0
			if i > 0 {
				if k >= workflow.Work.dag[i] {
					return errors.New("pipeline sequence is not a DAG")
				}
			}
			k = workflow.Work.dag[i]
		} else if workflow.Work.dag[i] == 0 {
			workflow.Work.isa[i] = 1
		} else {
			workflow.Work.isa[i] = -1
		}
		pre = workflow.Work.dag[i]
	}
	return nil
}

// dag returns the normalized DAG vertex for the given pipeline sequence order.
func dag(vertex int) int {
	return vertex*JobIdSpan + 1
}

/*

The standard operating procedure (SOP) table must have 5 column names, tab-delimited, and values cannot be empty.

Column names:

1. WorkflowID
2. AppID
3. Code
4. Slot
5. Filler

Column definitions:

1: Workflow ID
2: Application ID or unique name
3: Code to type the slot-filler context:
	R: Parameter required by the application
    O: Optional parameter
    F: If the boolean flag name is declared, it will be declared true, and the filler value is a dot
    S: Use for Slurm long option active argument, as for slot=account and filler=<account:string> DON'T USE error and output
    P: Use for PBS short option active argument, as for slot=A and filler=<account:string> DON'T USE e and o
	.: Use this to confirm the end of the SOP application record
4: Slot name is the application's command line parameter name, a value is expected unless is the end of the record.
5: This is the filler value associated with the slot, and it can be a dot only when the code is [O|F]

When using Slurm and PBS, the following active commands are automatically included, and there is no need to declare
in the SOP.

The Slurm active commands automatically included are:

- Stderr: --error
- Stdout: --output
- Job name: --job-name

The PBS active commands automatically included are:

- Stderr: -e
- Stdout: -o
- Job name: -N

The following is the SOP file for the mock test. Note that 3 dots mark the end of application records:

WorkflowID	AppID	Code	Slot	Filler
w10101	b32001	O	L	.
w10101	b32001	R	-i	{{DATA_PATH}}/{{BTC_USD_1_MIN_CSV_FILE}}
w10101	b32001	R	-A	{{EVENT_ART_PATH}}
w10101	b32001	.	.	.
w10101	b32002	O	L	.
w10101	b32002	R	-A	{{EVENT_ART_PATH}}
w10101	b32002	.	.	.
w10101	b32003	O	L	.
w10101	b32003	R	-A	{{EVENT_ART_PATH}}
w10101	b32003	R	-f	low
w10101	b32003	.	.	.
w10101	b32003	O	L	.
w10101	b32003	R	-A	{{EVENT_ART_PATH}}
w10101	b32003	R	-f	high
w10101	b32003	.	.	.
w10101	b32004	O	L	.
w10101	b32004	R	-A	{{EVENT_ART_PATH}}
w10101	b32004	.	.	.
w10101	b32005	O	L	.
w10101	b32005	R	-t	09:00:00^17:00:00^23:00:00
w10101	b32005	R	-o	btc_morning.tsv^btc_afternoon.tsv^btc_evening.tsv
w10101	b32005	R	-A	{{EVENT_ART_PATH}}
w10101	b32005	.	.	.

The above example requires the following metadata file and the config parameters below:

w10101	activity	Refines the BTC data, converts timestamps to dates and computes basic stats.
w10101	workflow	a60100 b32001 b32002 b32003 b32003 b32004 b32005
w10101	pipeline	1 0 2 1 0 3 2 0 4 2 0 5 4 3 0 6 5 0

BASE_PATH	<basePath>
WORKFLOW_SEQUENCE	<workflowIDs>
EVENT_TIME_TAG	<eventTimeTag>
BTC_USD_1_MIN_CSV_FILE	{{btcUsdMinCsvInpFile>}}

Filler parameters can be variables, e.g. {{BTC_USD_1_MIN_CSV_FILE}} declared in the metadata frame. Parameters delimited
with the '^' character are treated as parallel arrays. For example:

09:00:00^17:00:00^23:00:00
btc_morning.tsv^btc_afternoon.tsv^btc_evening.tsv

A case like this implies running 3 parallel job scripts with the same configuration.

The job scripts are generated, submitted, and monitored by the pipeline runner program. Each job script has its own
log file. The scripts, logfiles, and declarations make a journal hub of evidence about goals and results.

Test result using a maximum of 2 go routines:

=== RUN   TestWorkflow_RunScripts
Inf: 2026/02/20 13:45:57.597712 standby jobs: 8
Inf: 2026/02/20 13:45:57.597946 worker Go routine=[1] setting job: j01001_w10101_b60101
Inf: 2026/02/20 13:45:57.597970 worker Go routine=[0] setting job: j02001_w10101_b60102
Inf: 2026/02/20 13:46:04.554607 worker Go routine=[1] running job: j01001_w10101_b60101
Inf: 2026/02/20 13:46:04.554707 worker Go routine=[1] success job: j01001_w10101_b60101
Inf: 2026/02/20 13:46:04.554715 worker Go routine=[1] setting job: j03001_w10101_b60103
Inf: 2026/02/20 13:46:27.467956 worker Go routine=[0] running job: j02001_w10101_b60102
Inf: 2026/02/20 13:46:27.468023 worker Go routine=[0] success job: j02001_w10101_b60102
Inf: 2026/02/20 13:46:27.468030 worker Go routine=[0] setting job: j04001_w10101_b60103
Inf: 2026/02/20 13:46:29.623747 worker Go routine=[0] running job: j04001_w10101_b60103
Inf: 2026/02/20 13:46:29.623852 worker Go routine=[0] success job: j04001_w10101_b60103
Inf: 2026/02/20 13:46:29.623862 worker Go routine=[0] setting job: j05001_w10101_b60104
Inf: 2026/02/20 13:46:30.662591 worker Go routine=[1] running job: j03001_w10101_b60103
Inf: 2026/02/20 13:46:30.662698 worker Go routine=[1] success job: j03001_w10101_b60103
Inf: 2026/02/20 13:46:30.662707 worker Go routine=[1] setting job: j06001_w10101_b60105
Inf: 2026/02/20 13:46:31.767305 worker Go routine=[0] running job: j05001_w10101_b60104
Inf: 2026/02/20 13:46:31.767394 worker Go routine=[0] success job: j05001_w10101_b60104
Inf: 2026/02/20 13:46:31.767401 worker Go routine=[0] setting job: j06002_w10101_b60105
Inf: 2026/02/20 13:46:31.815232 worker Go routine=[0] running job: j06002_w10101_b60105
Inf: 2026/02/20 13:46:31.815339 worker Go routine=[0] success job: j06002_w10101_b60105
Inf: 2026/02/20 13:46:31.815347 worker Go routine=[0] setting job: j06003_w10101_b60105
Inf: 2026/02/20 13:46:31.832239 worker Go routine=[0] running job: j06003_w10101_b60105
Inf: 2026/02/20 13:46:31.832367 worker Go routine=[0] success job: j06003_w10101_b60105
Inf: 2026/02/20 13:46:32.697599 worker Go routine=[1] running job: j06001_w10101_b60105
Inf: 2026/02/20 13:46:32.697693 worker Go routine=[1] success job: j06001_w10101_b60105
Inf: 2026/02/20 13:46:32.697707 closed jobs: 8
Inf: 2026/02/20 13:46:32.697716 number of warnings caught: 0
Inf: 2026/02/20 13:46:32.697739 run time just before closing: 35.10139825
Inf: 2026/02/20 13:46:32.697741 end of process: complete
--- PASS: TestWorkflow_RunScripts (35.10s)
PASS

*/

// LoadSOP loads the SOP table, validates the expected 5 columns, and corresponds to the target workflow ID.
func (workflow *Workflow) LoadSOP(wid, fn string) error {
	if err := workflow.Work.SOP.LoadTable(fn); err != nil {
		return err
	}
	if workflow.Work.SOP.Columns() != 5 {
		return errors.New("SOP file must have 5 columns")
	}
	if workflow.Work.SOP.SearchColumn("WorkflowID") != SopWorkflowID {
		return fmt.Errorf("SOP table column %d is not named WorkflowID", SopWorkflowID+1)
	}
	if workflow.Work.SOP.SearchColumn("AppID") != SopAppID {
		return fmt.Errorf("SOP table column %d is not named AppID", SopAppID+1)
	}
	if workflow.Work.SOP.SearchColumn("Code") != SopCode {
		return fmt.Errorf("SOP table column %d is not named Code", SopCode+1)
	}
	if workflow.Work.SOP.SearchColumn("Slot") != SopSlot {
		return fmt.Errorf("SOP table column %d is not named Slot", SopSlot+1)
	}
	if workflow.Work.SOP.SearchColumn("Filler") != SopFiller {
		return fmt.Errorf("SOP table column %d is not named Filler", SopFiller+1)
	}
	if workflow.Work.SOP.SearchTable("WorkflowID", "AppID", wid) == "" {
		return fmt.Errorf("SOP file does not contain workflowID=[%s] expected in the first column", wid)
	}
	return nil
}

// Order validates the SOP table for the target workflow ID and returns the unique variables found in the filler column.
// The variables are expected to be declared in the metadata frame.
func (workflow *Workflow) Order(wid string) ([]string, error) {
	var unique []string
	vertex := 1 // 0 is the app running the apps
	nFillerParallelJobs := 0
	nApps := len(workflow.Work.app) - 1
	// r == 0 is the header row
	for r := 1; r < workflow.Work.SOP.Rows; r++ {
		if workflow.Work.SOP.Data[SopWorkflowID][r] != wid {
			continue
		}
		appID := workflow.Work.SOP.Data[SopAppID][r]
		code := workflow.Work.SOP.Data[SopCode][r]
		slot := workflow.Work.SOP.Data[SopSlot][r]
		filler := workflow.Work.SOP.Data[SopFiller][r]
		if code == SopCodeReq {
			if n := strings.Count(filler, "^"); n > 0 {
				nFillerParallelJobs = n + 1
			}
		}
		if code == SopCodeNull {
			if slot == SopCodeNull && filler == SopCodeNull {
				if vertex > nApps {
					return nil, fmt.Errorf("failed SOP order: expected %d number of apps but counted %d", nApps, vertex)
				}
				if runner.FindItem(workflow.Work.app, appID) == -1 {
					return nil, fmt.Errorf("failed SOP order: appID=[%s] not found", appID)
				}
				if workflow.Work.app[vertex] != appID {
					return nil, fmt.Errorf("failed SOP order: appID=[%s] has different order in the SOP file", appID)
				}
				if nFillerParallelJobs > 0 {
					workflow.fillerParallelJobs(nFillerParallelJobs, dag(vertex))
				}
				nFillerParallelJobs = 0
				vertex++
				continue
			}
			return nil, errors.New("failed SOP order: when code=[.] slot and filler are expected too")
		} else if code == SopCodePBS {
			workflow.SetBitFlag(ActPBS, true)
		} else if code == SopCodeSlurm {
			workflow.SetBitFlag(ActSlurm, true)
		}
		if strings.Contains(filler, "}}") {
			s := strings.SplitSeq(filler, "{{")
			for v := range s {
				t := strings.Split(v, "}}")
				if v != "" {
					if len(t) < 2 {
						return nil, errors.New("failed SOP order: invalid filler {{VARIABLE_NAME}} format")
					}
					unique = runner.InsertOnce(unique, t[0])
				}
			}
		}
	}
	if vertex-1 != nApps {
		return nil, fmt.Errorf("failed SOP order: expected %d number of apps but counted %d", nApps, vertex)
	}
	return unique, nil
}

// fillerParallelJobs updates the `isa` array to assign the number of parallel jobs for specific indices where `isa` equals 1.
// It updates only whenever nFillerParallelJobs > 1 otherwise the isa value remains 1.
func (workflow *Workflow) fillerParallelJobs(nFillerParallelJobs int, dag int) {
	// dag 1 0 2 1 0 3 2 0 4 2 0 5 4 3 0
	// isa 0 1 0 - 1 0 - 1 0 - 1 0 - - 1
	// isa 0 n 0 - n 0 - n 0 - n 0 - - n where n is the number of parallel jobs > 1
	workflow.Work.dMu.Lock()
	defer workflow.Work.dMu.Unlock()
	idx := 0
	for i := 0; i < len(workflow.Work.dag); i++ {
		if workflow.Work.dag[i] == dag {
			idx = i
			break
		}
	}
	for i := idx; i < len(workflow.Work.isa); i++ {
		if workflow.Work.isa[i] == 1 && nFillerParallelJobs > 1 {
			workflow.Work.isa[i] = nFillerParallelJobs
			return
		}
		if workflow.Work.dag[idx] == 0 {
			return
		}
	}
}

// JobIndices parses the job name to extract indices, returns the indices and the job state message, or error.
func (workflow *Workflow) JobIndices(jobName string) (int, int, int, string, error) {
	m := workflow.Jobs.SearchSlotS(jobName)
	if m == "" {
		return 0, 0, 0, m, fmt.Errorf("job name [%s] not found", jobName)
	}
	s := strings.Split(jobName, "_")
	if len(s) < 3 {
		return 0, 0, 0, m, fmt.Errorf("invalid job name: %s", jobName)
	}
	var e error
	var n, d, v int
	// Fast string slicing to extract the digits. Atoi handles "01" just fine.
	if n, e = strconv.Atoi(s[0][1:]); e != nil {
		return 0, 0, 0, m, e
	}
	if v, e = strconv.Atoi(s[0][1:3]); e != nil {
		return 0, 0, 0, m, e
	}
	if d, e = strconv.Atoi(s[0][3:]); e != nil {
		return 0, 0, 0, m, e
	}
	return n, v, d - 1, m, nil
}

// CreateScripts generates scripts based on the workflow configuration, dependencies, and metadata.
func (workflow *Workflow) CreateScripts(meta *runner.Framer) error {
	jix := 0
	var rq, fg, sl, pb, op []string
	type multiplex struct {
		key string
		par []string
	}
	n := 0
	box := make([]multiplex, 0)
	sfx := meta.SearchSlotS(runner.SuffixName) // ignored if the suffix name is an empty string
	scr := NewScriptor(
		meta.SearchSlotS(runner.EventActPath),
		meta.SearchSlotS(runner.EventFunPath),
		workflow.Work.app[0]) // the runner is 0
	// r == 0 is the header row
	for r := 1; r < workflow.Work.SOP.Rows; r++ {
		if workflow.Work.SOP.Data[SopWorkflowID][r] != workflow.Work.WID {
			continue
		}
		code := workflow.Work.SOP.Data[SopCode][r]
		slot := workflow.Work.SOP.Data[SopSlot][r]
		switch code {
		case SopCodeFlag:
			fg = append(fg, slot)
		case SopCodeReq:
			fl := sopRequired(workflow.Work.SOP.Data[SopFiller][r], meta)
			s := strings.Split(fl, "^")
			if len(s) > 1 {
				if n == 0 {
					n = len(s)
				}
				if n != len(s) {
					return errors.New("inconsistent number of parallel array items")
				}
				var b multiplex
				b.key = slot
				b.par = s
				box = append(box, b)
			} else {
				rq = append(rq, fmt.Sprintf("%s\t%s", slot, fl))
			}
		case SopCodeSlurm:
			fl := workflow.Work.SOP.Data[SopFiller][r]
			sl = append(sl, fmt.Sprintf("%s\t%s", slot, fl))
		case SopOptional:
			fl := sopRequired(workflow.Work.SOP.Data[SopFiller][r], meta)
			op = append(op, fmt.Sprintf("%s\t%s", slot, fl))
		case SopCodePBS:
			fl := workflow.Work.SOP.Data[SopFiller][r]
			pb = append(pb, fmt.Sprintf("%s\t%s", slot, fl))
		case SopCodeNull:
			if len(sl) > 0 && len(pb) > 0 {
				return errors.New("cannot process mixed use of PBS and Slurm parameters")
			}
			if n == 0 {
				n++
			}
			for pix := 0; pix < n; pix++ {
				app := workflow.Work.SOP.Data[SopAppID][r]
				aix := runner.FindItem(workflow.Work.app, app)
				if aix == -1 {
					return fmt.Errorf("appID=[%s] not found", app)
				}
				if pix == 0 {
					jix++
				}
				scr.NewJobName(JobIdSpan*jix+pix+1, aix, pix, workflow.Work.WID, app, sfx)
				scr.NewHeader()
				scr.SlurmParameters(sl)
				scr.PbsParameters(pb)
				scr.BinPath(meta.SearchSlotS(runner.EnvAppsPath), app)
				scr.Flags(fg)
				scr.ReqParameters(rq)
				scr.OptParameters(op)
				for i := range box {
					idx := runner.FindItem(workflow.Work.dag, aix*JobIdSpan+1)
					if idx == -1 {
						return fmt.Errorf("dag index not found: %d", aix*JobIdSpan+1)
					}
					if i == 0 {
						for j := idx; j < len(workflow.Work.isa); j++ {
							if workflow.Work.isa[j] > 0 {
								if workflow.Work.isa[j] != n {
									return fmt.Errorf("isa parallel jobs=[%d] but expecting: %d", workflow.Work.isa[j], n)
								}
							}
						}
					}
					scr.BoxParameters(box[i].key, box[i].par[pix])
				}
				scr.LogFile(op)
				if err := scr.SaveScript(); err != nil {
					return err
				}
				workflow.Jobs.Insert(scr.jobName, runner.JobStandBy)
			}
			rq, op, fg, sl, pb = nil, nil, nil, nil, nil
			box = box[:0]
			n = 0
		}
	}
	return nil
}

// sopRequired returns the processed filler; replaces all the {{VARIABLES}} with correspondent metadata.
func sopRequired(filler string, meta *runner.Framer) string {
	if strings.Contains(filler, "{{") {
		var u []string
		s := strings.SplitSeq(filler, "{{")
		for v := range s {
			t := strings.Split(v, "}}") // has been checked before
			u = runner.InsertOnce(u, t[0])
		}
		for _, variableName := range u {
			parameter := meta.SearchSlotS(variableName) // it exists, has been checked before
			filler = strings.ReplaceAll(filler, "{{"+variableName+"}}", parameter)
		}
	}
	return filler
}
