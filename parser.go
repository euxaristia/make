package main

import "strings"

func stripComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) && line[i+1] == '#' {
			i++
			continue
		}
		if line[i] == '$' && i+1 < len(line) {
			var closer byte
			if line[i+1] == '(' {
				closer = ')'
			} else if line[i+1] == '{' {
				closer = '}'
			}
			if closer != 0 {
				depth := 1
				i += 2
				for i < len(line) && depth > 0 {
					if line[i] == '(' || line[i] == '{' {
						depth++
					} else if line[i] == closer {
						depth--
					}
					i++
				}
				continue
			}
		}
		if line[i] == '#' {
			return strings.TrimRight(line[:i], " \t")
		}
	}
	return line
}

func Parse(content string, dag *Dag) string {
	var curTgts []string
	var phonyTargets []string
	var pendingRecipe string

	lines := strings.Split(content, "\n")
	li := 0
	nlines := len(lines)

	for li < nlines {
		rawLine := lines[li]

		if len(rawLine) > 0 && rawLine[0] == '\t' {
			trimmed := trim(rawLine)
			if len(trimmed) > 0 && len(curTgts) > 0 {
				endsBS := trimmed[len(trimmed)-1] == '\\'
				if endsBS {
					cmd := trimmed[:len(trimmed)-1]
					pendingRecipe += cmd + "\n"
				} else {
					recipe := pendingRecipe + trimmed
					for _, tgt := range curTgts {
						dag.AddRecipe(tgt, recipe)
					}
					pendingRecipe = ""
				}
			}
			li++
			continue
		}

		if pendingRecipe != "" {
			t := trim(pendingRecipe)
			if len(curTgts) > 0 && len(t) > 0 {
				for _, tgt := range curTgts {
					dag.AddRecipe(tgt, t)
				}
			}
			pendingRecipe = ""
		}

		trimmed := trim(rawLine)
		trimmed = stripComment(trimmed)
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
				curTgts = nil
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
			curTgts = nil
			li++
			continue
		}

		// Rule line: target: prereqs
		if colonPos := strings.IndexByte(trimmed, ':'); colonPos >= 0 {
			afterColon := trimmed[colonPos+1:]
			if len(afterColon) > 0 && afterColon[0] == '=' {
				curTgts = nil
				li++
				continue
			}
			targetPartRaw := trimmed[:colonPos]
			targetPart := trim(targetPartRaw)
			expander := &Expander{Vars: dag.Variables}
			expandedTargets := splitWS(expander.Simple(targetPart))
			if len(expandedTargets) == 0 {
				li++
				continue
			}
			trimmedPrereqs := trim(afterColon)
			expandedPrereqs := expander.Simple(trimmedPrereqs)
			curTgts = expandedTargets

			for _, tgt := range expandedTargets {
				isPhony := tgt == ".PHONY"
				dag.EnsureNode(tgt, isPhony)

				if tgt == ".PHONY" {
					for _, name := range splitWS(expandedPrereqs) {
						phonyTargets = append(phonyTargets, name)
						dag.EnsureNode(name, true)
					}
				} else if tgt != ".SUFFIXES" && (len(tgt) == 0 || tgt[0] != '.') && dag.DefaultTarget == "" {
					dag.SetDefault(tgt)
				}

				if len(expandedPrereqs) > 0 && tgt != ".PHONY" {
					for _, prereq := range splitWS(expandedPrereqs) {
						dag.AddPrereq(tgt, prereq)
					}
				}
			}
		}
		li++
	}

	if pendingRecipe != "" {
		t := trim(pendingRecipe)
		if len(curTgts) > 0 && len(t) > 0 {
			for _, tgt := range curTgts {
				dag.AddRecipe(tgt, t)
			}
		}
	}

	for _, name := range phonyTargets {
		if n, ok := dag.Nodes[name]; ok {
			n.IsPhony = true
		}
	}

	return ""
}
