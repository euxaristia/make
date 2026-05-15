package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func isVarChar(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isWS(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func trim(s string) string {
	return strings.Trim(s, " \t\n\r")
}

func isIdentifier(s string) bool {
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

func splitWS(s string) []string {
	return strings.Fields(s)
}

func shellCapture(cmd string) string {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(string(out)), " ")
}

func wildcardExpand(text string) string {
	var result []string
	for _, pattern := range strings.Fields(text) {
		if !strings.ContainsAny(pattern, "*?") {
			result = append(result, pattern)
			continue
		}
		matches, err := filepath.Glob(pattern)
		if err == nil {
			result = append(result, matches...)
		}
	}
	return strings.Join(result, " ")
}

type AutoVars struct {
	Target  string
	Prereqs []string
}

type Expander struct {
	Vars map[string]string
}

func (e *Expander) Simple(text string) string {
	return e.expand(text, nil, make(map[string]bool))
}

func (e *Expander) WithAuto(text string, auto *AutoVars) string {
	return e.expand(text, auto, make(map[string]bool))
}

func (e *Expander) expand(text string, auto *AutoVars, expanding map[string]bool) string {
	var b strings.Builder
	i := 0
	n := len(text)

	for i < n {
		c := text[i]
		if c != '$' {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= n {
			b.WriteByte('$')
			i++
			continue
		}
		nc := text[i+1]
		if nc == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}
		if nc == '(' || nc == '{' {
			name, args, consumed := parseFuncCall(text, i+1, nc)
			if name != "" {
				b.WriteString(e.callFunc(name, args, auto, expanding))
				i += consumed
				continue
			}
			i++
			continue
		}
		if auto != nil {
			switch nc {
			case '@':
				b.WriteString(auto.Target)
				i += 2
				continue
			case '<':
				if len(auto.Prereqs) > 0 {
					b.WriteString(auto.Prereqs[0])
				}
				i += 2
				continue
			case '^':
				seen := make(map[string]bool)
				var dedup []string
				for _, p := range auto.Prereqs {
					if !seen[p] {
						seen[p] = true
						dedup = append(dedup, p)
					}
				}
				b.WriteString(strings.Join(dedup, " "))
				i += 2
				continue
			case '+':
				b.WriteString(strings.Join(auto.Prereqs, " "))
				i += 2
				continue
			case '?':
				targetFI, _ := os.Stat(auto.Target)
				var newer []string
				for _, p := range auto.Prereqs {
					pFI, err := os.Stat(p)
					if err != nil || targetFI == nil || pFI.ModTime().After(targetFI.ModTime()) {
						newer = append(newer, p)
					}
				}
				b.WriteString(strings.Join(newer, " "))
				i += 2
				continue
			case '*':
				i += 2
				continue
			}
		}
		if isVarChar(nc) {
			name, consumed := parseSimpleVar(text, i+1)
			if name != "" {
				if !expanding[name] {
					raw := e.Vars[name]
					expanding[name] = true
					b.WriteString(e.expand(raw, auto, expanding))
					delete(expanding, name)
				}
				i += consumed
				continue
			}
		}
		b.WriteByte('$')
		i++
	}
	return b.String()
}

func parseSimpleVar(text string, start int) (string, int) {
	end := start
	for end < len(text) && isVarChar(text[end]) {
		end++
	}
	if end > start {
		return text[start:end], 1 + (end - start)
	}
	return "", 0
}

func parseFuncCall(text string, start int, opener byte) (name, args string, consumed int) {
	closer := byte(')')
	if opener == '{' {
		closer = '}'
	}
	depth := 1
	i := start + 1
	n := len(text)
	for i < n && depth > 0 {
		c := text[i]
		if c == opener {
			depth++
		} else if c == closer {
			depth--
		}
		i++
	}
	if depth != 0 {
		return "", "", 0
	}
	full := text[start+1 : i-1]
	consumed = (i - start) + 1

	// Substitution reference: $(name:s1=s2)
	if colonIdx := strings.IndexByte(full, ':'); colonIdx >= 0 {
		prefix := full[:colonIdx]
		if isIdentifier(trim(prefix)) {
			suffix := full[colonIdx+1:]
			if strings.Contains(suffix, "=") {
				return full, "", consumed
			}
		}
	}

	// Function call shape: $(func args) or $(func,args)
	if spaceIdx := strings.IndexByte(full, ' '); spaceIdx >= 0 {
		return full[:spaceIdx], full[spaceIdx+1:], consumed
	}
	if commaIdx := strings.IndexByte(full, ','); commaIdx >= 0 {
		return full[:commaIdx], full[commaIdx+1:], consumed
	}
	return full, "", consumed
}

func (e *Expander) callFunc(name, args string, auto *AutoVars, expanding map[string]bool) string {
	name = trim(name)
	args = trim(args)
	switch name {
	case "wildcard":
		return wildcardExpand(args)
	case "shell":
		return shellCapture(args)
	default:
		// Substitution reference: $(VAR:s1=s2)
		if colonIdx := strings.IndexByte(name, ':'); colonIdx >= 0 {
			pattern := name[colonIdx+1:]
			eqIdx := strings.IndexByte(pattern, '=')
			if eqIdx < 0 {
				return ""
			}
			varName := trim(name[:colonIdx])
			s1 := pattern[:eqIdx]
			s2 := pattern[eqIdx+1:]

			value := ""
			if !expanding[varName] {
				raw := e.Vars[varName]
				expanding[varName] = true
				value = e.expand(raw, auto, expanding)
				delete(expanding, varName)
			}
			s1exp := e.expand(s1, auto, expanding)
			s2exp := e.expand(s2, auto, expanding)

			words := splitWS(value)
			var result []string
			for _, w := range words {
				if len(s1exp) == 0 {
					result = append(result, w+s2exp)
				} else if len(s1exp) <= len(w) && w[len(w)-len(s1exp):] == s1exp {
					result = append(result, w[:len(w)-len(s1exp)]+s2exp)
				} else {
					result = append(result, w)
				}
			}
			return strings.Join(result, " ")
		}
		// Plain variable lookup
		if expanding[name] {
			return ""
		}
		raw := e.Vars[name]
		expanding[name] = true
		result := e.expand(raw, auto, expanding)
		delete(expanding, name)
		return result
	}
}


