package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CliArgs struct {
	Target      string
	Makefile    string
	Jobs        int
	KeepGoing   bool
	IgnoreErrs  bool
	Silent      bool
	Question    bool
	PrintDB     bool
	DryRun      bool
	Touch       bool
	EnvOverride bool
	Help        bool
	Version     bool
	Overrides   map[string]string
}

func parseArgs(argv []string) (*CliArgs, string) {
	a := &CliArgs{
		Jobs:      1,
		Overrides: make(map[string]string),
	}
	i := 0
	n := len(argv)

	for i < n {
		s := argv[i]
		switch s {
		case "-f":
			i++
			if i >= n {
				return nil, "-f requires an argument"
			}
			a.Makefile = argv[i]
		case "-j":
			i++
			if i >= n {
				return nil, "-j requires an argument"
			}
			v, err := strconv.Atoi(argv[i])
			if err != nil || v < 1 {
				return nil, "invalid -j value"
			}
			a.Jobs = v
		case "-k":
			a.KeepGoing = true
		case "-S":
			a.KeepGoing = false
		case "-i":
			a.IgnoreErrs = true
		case "-s":
			a.Silent = true
		case "-q":
			a.Question = true
		case "-p":
			a.PrintDB = true
		case "-n":
			a.DryRun = true
		case "-t":
			a.Touch = true
		case "-e":
			a.EnvOverride = true
		case "-r":
			// no-op
		case "-h", "--help":
			a.Help = true
		case "--version":
			a.Version = true
		default:
			if len(s) > 0 && s[0] == '-' {
				return nil, "unknown option: " + s
			}
			if eqIdx := strings.IndexByte(s, '='); eqIdx >= 0 {
				name := s[:eqIdx]
				if isValidName(name) {
					value := s[eqIdx+1:]
					a.Overrides[name] = value
				} else {
					a.Target = s
				}
			} else {
				a.Target = s
			}
		}
		i++
	}
	return a, ""
}

func isValidName(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isVarChar(s[i]) {
			return false
		}
	}
	return true
}

func printUsage() {
	fmt.Println("Usage: make [target] [NAME=value ...] [-f FILE] [-j N] [-eikSnpqrst]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -f FILE   Read FILE as the makefile (default: Makefile, then makefile)")
	fmt.Println("  -j N      Run up to N recipes in parallel (default: 1)")
	fmt.Println("  -e        Environment variables override Makefile assignments")
	fmt.Println("  -i        Ignore errors from commands")
	fmt.Println("  -k        Keep going after errors")
	fmt.Println("  -S        Cancel a prior -k (errors stop the build)")
	fmt.Println("  -n        Dry run (print commands but don't execute)")
	fmt.Println("  -p        Print database (rules and variables)")
	fmt.Println("  -q        Question mode (exit 0 if up to date, 1 otherwise)")
	fmt.Println("  -r        Disable built-in rules (no-op for compatibility)")
	fmt.Println("  -s        Silent mode (don't echo commands)")
	fmt.Println("  -t        Touch targets instead of running recipes")
	fmt.Println("  -h        Show this help")
	fmt.Println("  --version Show version")
}

func main() {
	args, errMsg := parseArgs(os.Args[1:])
	if errMsg != "" {
		fmt.Fprintf(os.Stderr, "make: %s\n", errMsg)
		os.Exit(2)
	}

	if args.Help {
		printUsage()
		return
	}

	if args.Version {
		fmt.Println("make 0.2.0")
		return
	}

	// Resolve makefile
	makefile := args.Makefile
	if makefile == "" {
		if _, err := os.Stat("Makefile"); err == nil {
			makefile = "Makefile"
		} else if _, err := os.Stat("makefile"); err == nil {
			makefile = "makefile"
		} else {
			fmt.Fprintf(os.Stderr, "make: *** No makefile found.\n")
			os.Exit(2)
		}
	}

	content, err := os.ReadFile(makefile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "make: *** Cannot open %s\n", makefile)
		os.Exit(2)
	}

	dag := NewDag()

	// Import environment variables before parsing
	for _, kv := range os.Environ() {
		if eqIdx := strings.IndexByte(kv, '='); eqIdx >= 0 {
			k := kv[:eqIdx]
			v := kv[eqIdx+1:]
			dag.SetVariable(k, v)
		}
	}

	// Apply command-line macro overrides before parsing
	for k, v := range args.Overrides {
		dag.SetOverride(k, v)
	}

	// Parse
	if errMsg := Parse(string(content), dag); errMsg != "" {
		fmt.Fprintf(os.Stderr, "make: %s\n", errMsg)
		os.Exit(2)
	}

	// -e: env vars override Makefile assignments (but not command-line overrides)
	if args.EnvOverride {
		for _, kv := range os.Environ() {
			if eqIdx := strings.IndexByte(kv, '='); eqIdx >= 0 {
				k := kv[:eqIdx]
				v := kv[eqIdx+1:]
				dag.SetVariable(k, v)
			}
		}
	}

	// Cycle detection
	if cycle := dag.DetectCycle(); cycle != nil {
		fmt.Fprintf(os.Stderr, "make: *** Circular dependency: %s\n", strings.Join(cycle, " -> "))
		os.Exit(2)
	}

	// Determine target
	buildTarget := args.Target
	if buildTarget != "" {
		if _, ok := dag.Nodes[buildTarget]; !ok {
			fmt.Fprintf(os.Stderr, "make: *** No rule to make %s\n", buildTarget)
			os.Exit(2)
		}
	} else {
		if dag.DefaultTarget == "" {
			fmt.Fprintf(os.Stderr, "make: *** No default target.\n")
			os.Exit(2)
		}
		buildTarget = dag.DefaultTarget
	}

	nodes := dag.Order(buildTarget)

	// -p: print database
	if args.PrintDB {
		fmt.Println("# Variables")
		for k, v := range dag.Variables {
			fmt.Printf("%s = %s\n", k, v)
		}
		fmt.Println("")
		fmt.Println("# Rules")
		for _, nd := range nodes {
			if len(nd.Prereqs) == 0 && len(nd.Recipes) == 0 {
				continue
			}
			line := nd.Target + ":"
			for _, p := range nd.Prereqs {
				line += " " + p
			}
			fmt.Println(line)
			for _, r := range nd.Recipes {
				fmt.Println("\t" + r)
			}
		}
		return
	}

	// -q: question mode
	if args.Question {
		for _, nd := range nodes {
			if needsRebuild(nd) {
				os.Exit(1)
			}
		}
		return
	}

	// Build job list
	jobs := make([]Job, 0, len(nodes))
	for _, nd := range nodes {
		recipes := make([]string, len(nd.Recipes))
		copy(recipes, nd.Recipes)
		prereqs := make([]string, len(nd.Prereqs))
		copy(prereqs, nd.Prereqs)
		jobs = append(jobs, Job{
			Target:  nd.Target,
			Recipes: recipes,
			Prereqs: prereqs,
			IsPhony: nd.IsPhony,
		})
	}

	exec := &Executor{
		NumJobs:      args.Jobs,
		KeepGoing:    args.KeepGoing,
		IgnoreErrors: args.IgnoreErrs,
		Silent:       args.Silent,
		DryRun:       args.DryRun,
		Touch:        args.Touch,
		Vars:         dag.Variables,
		Target:       buildTarget,
	}
	errors := exec.Run(jobs)
	if errors > 0 {
		os.Exit(1)
	}
}
