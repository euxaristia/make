package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Job struct {
	Target  string
	Recipes []string
	Prereqs []string
	IsPhony bool
}

type Executor struct {
	NumJobs      int
	KeepGoing    bool
	IgnoreErrors bool
	Silent       bool
	DryRun       bool
	Touch        bool
	Vars         map[string]string
	Target       string
}

func needsRebuild(node *DagNode) bool {
	if node.IsPhony {
		return true
	}
	targetMtime := mtime(node.Target)
	if targetMtime == 0 {
		return true
	}
	for _, prereq := range node.Prereqs {
		pm := mtime(prereq)
		if pm == 0 || pm > targetMtime {
			return true
		}
	}
	return false
}

type executorState struct {
	sync.Mutex

	byName     map[string]Job
	remaining  map[string]int
	dependents map[string][]string
	completed  map[string]bool
	failed     map[string]bool

	ready    []string
	inFlight int
	errors   int
	stop     bool
	wg       sync.WaitGroup
	done     chan struct{}
}

func (e *Executor) Run(jobs []Job) int {
	st := &executorState{
		byName:     make(map[string]Job),
		remaining:  make(map[string]int),
		dependents: make(map[string][]string),
		completed:  make(map[string]bool),
		failed:     make(map[string]bool),
		done:       make(chan struct{}),
	}

	inSet := make(map[string]bool)
	for _, j := range jobs {
		inSet[j.Target] = true
	}

	for _, j := range jobs {
		st.byName[j.Target] = j
		count := 0
		for _, p := range j.Prereqs {
			if inSet[p] {
				count++
				st.dependents[p] = append(st.dependents[p], j.Target)
			}
		}
		st.remaining[j.Target] = count
	}

	needsBuild := false
	for _, j := range jobs {
		if len(j.Recipes) > 0 && needsRebuild(&DagNode{Target: j.Target, Prereqs: j.Prereqs, IsPhony: j.IsPhony}) {
			needsBuild = true
			break
		}
	}
	if !needsBuild {
		fmt.Printf("mkultra: nothing to be done for '%s'\n", e.Target)
		return 0
	}

	var roots []string
	for _, j := range jobs {
		if st.remaining[j.Target] == 0 {
			roots = append(roots, j.Target)
		}
	}
	for _, name := range roots {
		e.enqueue(st, name)
	}

	e.dispatch(st)

	go func() {
		st.wg.Wait()
		close(st.done)
	}()

	<-st.done
	return st.errors
}

func (e *Executor) enqueue(st *executorState, target string) {
	st.Lock()
	defer st.Unlock()

	if st.completed[target] {
		return
	}
	job, ok := st.byName[target]
	if !ok {
		return
	}

	for _, p := range job.Prereqs {
		if st.failed[p] {
			st.failed[target] = true
			if len(job.Recipes) > 0 {
				fmt.Fprintf(os.Stderr, "mkultra: Target '%s' not remade because of errors\n", target)
			}
			e.completeLocked(st, target)
			return
		}
	}

	if len(job.Recipes) == 0 || !e.jobNeedsRebuild(st, job) {
		e.completeLocked(st, target)
	} else {
		st.ready = append(st.ready, target)
	}
}

func (e *Executor) completeLocked(st *executorState, target string) {
	if st.completed[target] {
		return
	}
	st.completed[target] = true
	for _, dep := range st.dependents[target] {
		st.remaining[dep]--
		if st.remaining[dep] == 0 && !st.stop {
			e.enqueueLocked(st, dep)
		}
	}
}

func (e *Executor) enqueueLocked(st *executorState, target string) {
	if st.completed[target] {
		return
	}
	job, ok := st.byName[target]
	if !ok {
		return
	}

	for _, p := range job.Prereqs {
		if st.failed[p] {
			st.failed[target] = true
			if len(job.Recipes) > 0 {
				fmt.Fprintf(os.Stderr, "mkultra: Target '%s' not remade because of errors\n", target)
			}
			e.completeLocked(st, target)
			return
		}
	}

	if len(job.Recipes) == 0 || !e.jobNeedsRebuild(st, job) {
		e.completeLocked(st, target)
	} else {
		st.ready = append(st.ready, target)
	}
}

func (e *Executor) dispatch(st *executorState) {
	st.Lock()
	defer st.Unlock()
	e.dispatchLocked(st)
}

func (e *Executor) dispatchLocked(st *executorState) {
	if st.stop {
		return
	}
	for st.inFlight < e.NumJobs && len(st.ready) > 0 {
		target := st.ready[0]
		st.ready = st.ready[1:]
		job := st.byName[target]
		st.inFlight++
		st.wg.Add(1)
		go func(j Job) {
			defer st.wg.Done()

			ok := e.build(j)

			st.Lock()
			st.inFlight--
			if !ok {
				st.errors++
				st.failed[j.Target] = true
				if !e.KeepGoing && !e.IgnoreErrors {
					st.stop = true
				}
			}
			e.completeLocked(st, j.Target)
			e.dispatchLocked(st)
			st.Unlock()
		}(job)
	}
}

func (e *Executor) jobNeedsRebuild(st *executorState, j Job) bool {
	if j.IsPhony {
		return true
	}
	targetMtime := mtime(j.Target)
	if targetMtime == 0 {
		return true
	}
	for _, prereq := range j.Prereqs {
		pm := mtime(prereq)
		if pm == 0 || pm > targetMtime {
			return true
		}
	}
	return false
}

func (e *Executor) build(job Job) bool {
	auto := &AutoVars{Target: job.Target, Prereqs: job.Prereqs}
	exp := &Expander{Vars: e.Vars}

	if e.Touch {
		if !job.IsPhony {
			quoted := shellQuote(job.Target)
			cmd := "touch " + quoted
			if !e.Silent {
				fmt.Println(cmd)
			}
			execShell(cmd)
		}
		return true
	}

	for _, recipe := range job.Recipes {
		cmdRaw, silentPref, ignorePref, alwaysPref := parsePrefixes(recipe)
		suppressed := silentPref || e.Silent
		expanded := exp.WithAuto(cmdRaw, auto)

		if e.DryRun && !alwaysPref {
			if !suppressed {
				fmt.Println(expanded)
			}
			continue
		}
		if !suppressed {
			fmt.Println(expanded)
		}

		code := execShell(expanded)
		if code != 0 {
			if ignorePref || e.IgnoreErrors {
				fmt.Fprintf(os.Stderr, "mkultra: [%s] Error %d (ignored)\n", job.Target, code)
				continue
			}
			fmt.Fprintf(os.Stderr, "mkultra: *** [%s] Error %d\n", job.Target, code)
			return false
		}
	}
	return true
}

func parsePrefixes(recipe string) (cmd string, silent, ignoreErr, alwaysRun bool) {
	i := 0
	n := len(recipe)
	for i < n {
		c := recipe[i]
		switch c {
		case '@':
			silent = true
		case '-':
			ignoreErr = true
		case '+':
			alwaysRun = true
		default:
			return recipe[i:], silent, ignoreErr, alwaysRun
		}
		i++
	}
	return recipe[i:], silent, ignoreErr, alwaysRun
}

func shellQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, c := range s {
		if c == '\'' {
			b.WriteString("'\\''")
		} else {
			b.WriteRune(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func execShell(cmd string) int {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 127
	}
	return 0
}
