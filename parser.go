package main

import "strings"

func Parse(content string, dag *Dag) string {
	curTgt := ""
	var phonyTargets []string
	var pendingRecipe string

	lines := strings.Split(content, "\n")
	li := 0
	nlines := len(lines)

	for li < nlines {
		rawLine := lines[li]

		if len(rawLine) > 0 && rawLine[0] == '\t' {
			trimmed := trim(rawLine)
			if len(trimmed) > 0 && len(curTgt) > 0 {
				endsBS := trimmed[len(trimmed)-1] == '\\'
				if endsBS {
					cmd := trimmed[:len(trimmed)-1]
					pendingRecipe += cmd + "\n"
				} else {
					dag.AddRecipe(curTgt, pendingRecipe+trimmed)
					pendingRecipe = ""
				}
			}
			li++
			continue
		}

		if pendingRecipe != "" {
			t := trim(pendingRecipe)
			if len(curTgt) > 0 && len(t) > 0 {
				dag.AddRecipe(curTgt, t)
			}
			pendingRecipe = ""
		}

		trimmed := trim(rawLine)
		if len(trimmed) == 0 {
			li++
			continue
		}
		if trimmed[0] == '#' {
			li++
			continue
		}

		// := simply-expanded variable assignment
		if pos := strings.Index(trimmed, ":="); pos >= 0 {
			lhsRaw := trimmed[:pos]
			rhsRaw := trimmed[pos+2:]
			lhs := trim(lhsRaw)
			rhs := trim(rhsRaw)
			if len(lhs) > 0 && !strings.Contains(lhs, " ") {
				expanded := (&Expander{Vars: dag.Variables}).Simple(rhs)
				dag.SetVariable(lhs, expanded)
				curTgt = ""
				li++
				continue
			}
		}

		// =, ?=, += variable assignment
		if eqPos := strings.IndexByte(trimmed, '='); eqPos >= 0 {
			beforeEq := trimmed[:eqPos]
			bef := trim(beforeEq)
			if len(bef) > 0 && bef[len(bef)-1] == ':' {
				// This is := syntax, already handled above.
				li++
				continue
			}

			charBefore := byte(0)
			if eqPos > 0 {
				charBefore = trimmed[eqPos-1]
			}
			isQP := charBefore == '?' || charBefore == '+'
			lhsForStore := bef
			if isQP {
				lhsForStore = trim(bef[:len(bef)-1])
			}
			if len(lhsForStore) == 0 || strings.ContainsAny(lhsForStore, " :") {
				li++
				continue
			}
			rhsRaw := trimmed[eqPos+1:]
			rhs := trim(rhsRaw)
			switch charBefore {
			case '?':
				if _, ok := dag.Variables[lhsForStore]; !ok {
					dag.SetVariable(lhsForStore, rhs)
				}
			case '+':
				cur := dag.Variables[lhsForStore]
				sep := ""
				if cur != "" {
					sep = " "
				}
				dag.SetVariable(lhsForStore, cur+sep+rhs)
			default:
				dag.SetVariable(lhsForStore, rhs)
			}
			curTgt = ""
			li++
			continue
		}

		// Rule line: target: prereqs
		if colonPos := strings.IndexByte(trimmed, ':'); colonPos >= 0 {
			afterColon := trimmed[colonPos+1:]
			if len(afterColon) > 0 && afterColon[0] == '=' {
				curTgt = ""
				li++
				continue
			}
			targetPartRaw := trimmed[:colonPos]
			targetPart := trim(targetPartRaw)
			expander := &Expander{Vars: dag.Variables}
			expandedTarget := expander.Simple(targetPart)
			trimmedPrereqs := trim(afterColon)
			expandedPrereqs := expander.Simple(trimmedPrereqs)
			curTgt = expandedTarget

			isPhony := expandedTarget == ".PHONY"
			dag.EnsureNode(expandedTarget, isPhony)

			if expandedTarget == ".PHONY" {
				for _, name := range splitWS(expandedPrereqs) {
					phonyTargets = append(phonyTargets, name)
					dag.EnsureNode(name, true)
				}
			} else if expandedTarget != ".SUFFIXES" && (len(expandedTarget) == 0 || expandedTarget[0] != '.') && dag.DefaultTarget == "" {
				dag.SetDefault(expandedTarget)
			}

			if len(expandedPrereqs) > 0 && expandedTarget != ".PHONY" {
				for _, prereq := range splitWS(expandedPrereqs) {
					dag.AddPrereq(expandedTarget, prereq)
				}
			}
		}
		li++
	}

	if pendingRecipe != "" {
		t := trim(pendingRecipe)
		if len(curTgt) > 0 && len(t) > 0 {
			dag.AddRecipe(curTgt, t)
		}
	}

	for _, name := range phonyTargets {
		if n, ok := dag.Nodes[name]; ok {
			n.IsPhony = true
		}
	}

	return ""
}
