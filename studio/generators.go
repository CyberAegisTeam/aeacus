package main

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type RegexRequest struct {
	Mode                  string   `json:"mode"`
	Key                   string   `json:"key"`
	Separator             string   `json:"separator"`
	Values                []string `json:"values"`
	CaseSensitive         bool     `json:"caseSensitive"`
	FlexibleWhitespace    bool     `json:"flexibleWhitespace"`
	SpacesUnderscoresSame bool     `json:"spacesUnderscoresSame"`
	WholeLine             bool     `json:"wholeLine"`
	Operator              string   `json:"operator"`
	Number                int      `json:"number"`
	Maximum               int      `json:"maximum"`
}

type RegexResponse struct {
	Pattern string   `json:"pattern"`
	Tests   []string `json:"tests,omitempty"`
}

func buildRegex(request RegexRequest) (RegexResponse, error) {
	prefix, suffix := "", ""
	if request.WholeLine {
		prefix, suffix = "^", "$"
	}
	space := `[[:space:]]*`
	if !request.FlexibleWhitespace {
		space = " "
	}
	casePrefix := ""
	if !request.CaseSensitive {
		casePrefix = "(?i)"
	}
	escapeText := func(value string) string {
		value = regexp.QuoteMeta(value)
		if request.SpacesUnderscoresSame {
			value = strings.ReplaceAll(value, `\ `, `[_[:space:]]+`)
		} else if request.FlexibleWhitespace {
			value = strings.ReplaceAll(value, `\ `, `[[:space:]]+`)
		}
		return value
	}
	switch request.Mode {
	case "exact", "contains", "starts", "ends":
		if len(request.Values) == 0 {
			return RegexResponse{}, fmt.Errorf("at least one value is required")
		}
		parts := make([]string, 0, len(request.Values))
		for _, value := range request.Values {
			parts = append(parts, escapeText(value))
		}
		body := parts[0]
		if len(parts) > 1 {
			body = "(" + strings.Join(parts, "|") + ")"
		}
		if request.Mode == "exact" {
			prefix, suffix = "^", "$"
		}
		if request.Mode == "starts" {
			prefix = "^"
		}
		if request.Mode == "ends" {
			suffix = "$"
		}
		return RegexResponse{Pattern: casePrefix + prefix + body + suffix}, nil
	case "setting":
		if request.Key == "" || len(request.Values) == 0 {
			return RegexResponse{}, fmt.Errorf("setting name and value are required")
		}
		values := make([]string, 0, len(request.Values))
		for _, value := range request.Values {
			values = append(values, escapeText(value))
		}
		valuePattern := values[0]
		if len(values) > 1 {
			valuePattern = "(" + strings.Join(values, "|") + ")"
		}
		separator := space + `=` + space
		switch request.Separator {
		case "whitespace":
			separator = `[[:space:]]+`
		case "colon":
			separator = space + `:` + space
		case "either":
			separator = space + `([=:]|[[:space:]]+)` + space
		case "", "equals":
		default:
			return RegexResponse{}, fmt.Errorf("unknown setting separator %q", request.Separator)
		}
		return RegexResponse{Pattern: casePrefix + "^" + space + escapeText(request.Key) + separator + valuePattern + space + "$"}, nil
	case "integer":
		pattern, tests, err := integerRegex(request.Operator, request.Number, request.Maximum)
		return RegexResponse{Pattern: "^" + pattern + "$", Tests: tests}, err
	default:
		return RegexResponse{}, fmt.Errorf("unknown regex builder mode %q", request.Mode)
	}
}

func integerRegex(operator string, number, maximum int) (string, []string, error) {
	if number < 0 || maximum < 0 {
		return "", nil, fmt.Errorf("visual numeric patterns currently support non-negative integers")
	}
	var accepts func(int) bool
	switch operator {
	case ">":
		accepts = func(v int) bool { return v > number }
	case ">=":
		accepts = func(v int) bool { return v >= number }
	case "<":
		accepts = func(v int) bool { return v < number }
	case "<=":
		accepts = func(v int) bool { return v <= number }
	case "between":
		if maximum < number {
			return "", nil, fmt.Errorf("maximum must be greater than or equal to minimum")
		}
		accepts = func(v int) bool { return v >= number && v <= maximum }
	case "=":
		accepts = func(v int) bool { return v == number }
	default:
		return "", nil, fmt.Errorf("unknown numeric operator")
	}
	minimum, upper, unbounded := 0, maximum, false
	switch operator {
	case ">":
		minimum, upper, unbounded = number+1, decimalCeiling(number), true
	case ">=":
		minimum, upper, unbounded = number, decimalCeiling(number), true
	case "<":
		minimum, upper = 0, number-1
	case "<=":
		minimum, upper = 0, number
	case "=":
		minimum, upper = number, number
	case "between":
		minimum, upper = number, maximum
	}
	values := decimalRangePatterns(minimum, upper)
	if unbounded {
		digits := len(strconv.Itoa(upper))
		values = append(values, fmt.Sprintf(`[1-9][0-9]{%d,}`, digits))
	}
	if len(values) == 0 {
		return `a^`, nil, nil
	}
	tests := []string{}
	for _, value := range []int{max(0, number-1), number, number + 1, maximum} {
		tests = append(tests, fmt.Sprintf("%d: %t", value, accepts(value)))
	}
	return "(" + strings.Join(values, "|") + ")", tests, nil
}

func decimalCeiling(value int) int {
	ceiling := 9
	for ceiling < value {
		ceiling = ceiling*10 + 9
	}
	return ceiling
}

func decimalRangePatterns(minimum, maximum int) []string {
	if maximum < minimum || maximum < 0 {
		return nil
	}
	if minimum < 0 {
		minimum = 0
	}
	patterns := []string{}
	for current := minimum; current <= maximum; {
		block := 1
		for next := 10; current%next == 0 && current+next-1 <= maximum; next *= 10 {
			block = next
		}
		if block == 1 {
			patterns = append(patterns, strconv.Itoa(current))
			current++
			continue
		}
		zeros := len(strconv.Itoa(block)) - 1
		prefix := current / block
		if prefix == 0 {
			patterns = append(patterns, fmt.Sprintf(`[0-9]{1,%d}`, zeros))
		} else {
			patterns = append(patterns, fmt.Sprintf(`%d[0-9]{%d}`, prefix, zeros))
		}
		current += block
	}
	return patterns
}

func generateForensicFiles(project *Project) map[string]string {
	files := map[string]string{}
	for qIndex := range project.Forensics {
		question := &project.Forensics[qIndex]
		question.Number = qIndex + 1
		var content strings.Builder
		if question.Title != "" {
			content.WriteString(question.Title + "\n\n")
		}
		if question.Intro != "" {
			content.WriteString(question.Intro + "\n\n")
		}
		for partIndex, part := range question.Parts {
			if len(question.Parts) > 1 {
				content.WriteString(fmt.Sprintf("Part %d: ", partIndex+1))
			}
			content.WriteString(part.Prompt + "\nANSWER:\n\n")
		}
		files[fmt.Sprintf("FQ%d.txt", question.Number)] = strings.TrimSpace(content.String()) + "\n"
	}
	return files
}

func synchronizeForensicChecks(project *Project) {
	filtered := make([]Check, 0, len(project.Config.Checks))
	for _, check := range project.Config.Checks {
		if !strings.HasPrefix(check.ID, "studio-fq-") {
			filtered = append(filtered, check)
		}
	}
	for qIndex, question := range project.Forensics {
		for partIndex, part := range question.Parts {
			if len(part.Answers) == 0 {
				continue
			}
			values := make([]string, 0, len(part.Answers))
			for _, answer := range part.Answers {
				value := regexp.QuoteMeta(answer)
				if part.FlexibleWhitespace {
					value = strings.ReplaceAll(value, `\ `, `[[:space:]]+`)
				}
				values = append(values, value)
			}
			prefix := ""
			if !part.CaseSensitive {
				prefix = "(?i)"
			}
			pattern := prefix + `^[[:space:]]*ANSWER:[[:space:]]+(` + strings.Join(values, "|") + `)[[:space:]]*$`
			filtered = append(filtered, Check{
				ID: fmt.Sprintf("studio-fq-%d-%d", qIndex+1, partIndex+1), Category: "Forensics",
				Message: fmt.Sprintf("Forensic Question %d Part %d Correct", qIndex+1, partIndex+1), Points: 5,
				Pass: []Condition{{Type: "FileContainsRegex", Path: fmt.Sprintf("%s/Desktop/FQ%d.txt", homeForConfig(project.Config), qIndex+1), Value: pattern}},
			})
		}
	}
	project.Config.Checks = filtered
	ensureCheckIDs(&project.Config)
}

func synchronizeGeneratedChecks(project *Project) {
	synchronizeForensicChecks(project)
	filtered := make([]Check, 0, len(project.Config.Checks))
	for _, check := range project.Config.Checks {
		if !strings.HasPrefix(check.ID, "studio-user-") {
			filtered = append(filtered, check)
		}
	}
	for _, user := range project.Users {
		if user.Username == "" {
			continue
		}
		if user.RemovalPenalty {
			filtered = append(filtered, Check{ID: "studio-user-removal-" + slug(user.Username), Category: "Users / Groups", Message: "Authorized user " + user.Username + " has been removed", Points: -5, Pass: []Condition{{Type: "UserExistsNot", User: user.Username}}})
		}
		if user.PasswordCheck {
			condition := Condition{Type: "PasswordChanged", User: user.Username}
			if strings.Contains(strings.ToLower(project.Config.OS), "windows") {
				condition.After = "REPLACE_PASSWORD_CHANGE_DATE"
			} else {
				condition.Value = "REPLACE_PASSWORD_HASH"
			}
			filtered = append(filtered, Check{ID: "studio-user-password-" + slug(user.Username), Category: "Users / Groups", Message: "Changed password for user " + user.Username, Points: 1, Pass: []Condition{condition}})
		}
		if user.GroupChecks {
			for _, group := range user.Groups {
				filtered = append(filtered, Check{ID: "studio-user-group-" + slug(user.Username) + "-" + slug(group), Category: "Users / Groups", Message: "User " + user.Username + " is a member of " + group, Points: 1, Pass: []Condition{{Type: "UserInGroup", User: user.Username, Group: group}}})
			}
		}
	}
	project.Config.Checks = filtered
	ensureCheckIDs(&project.Config)
}

func homeForConfig(config ScoringConfig) string {
	if strings.Contains(strings.ToLower(config.OS), "windows") {
		return `C:\Users\` + config.User
	}
	return "/home/" + config.User
}

func generateSetupScripts(project Project) []ProjectScript {
	users := append([]ImageUser(nil), project.Users...)
	sort.SliceStable(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	var linux, windows strings.Builder
	linux.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n\n")
	windows.WriteString("#Requires -RunAsAdministrator\n$ErrorActionPreference = 'Stop'\n\n")
	for _, user := range users {
		if user.Username == "" {
			continue
		}
		shell := user.Shell
		if shell == "" {
			shell = "/bin/bash"
		}
		linux.WriteString(fmt.Sprintf("if ! id %s >/dev/null 2>&1; then useradd -m -s %s %s; fi\necho %s:%s | chpasswd\n", shellQuote(user.Username), shellQuote(shell), shellQuote(user.Username), shellQuote(user.Username), shellQuote(user.Password)))
		if user.Role == "admin" || user.Role == "primary" {
			linux.WriteString(fmt.Sprintf("usermod -aG sudo %s\n", shellQuote(user.Username)))
		}
		for _, group := range user.Groups {
			linux.WriteString(fmt.Sprintf("getent group %s >/dev/null || groupadd %s\nusermod -aG %s %s\n", shellQuote(group), shellQuote(group), shellQuote(group), shellQuote(user.Username)))
		}
		windows.WriteString(fmt.Sprintf("if (-not (Get-LocalUser -Name %s -ErrorAction SilentlyContinue)) { $pw = ConvertTo-SecureString %s -AsPlainText -Force; New-LocalUser -Name %s -Password $pw -FullName %s }\n", psQuote(user.Username), psQuote(user.Password), psQuote(user.Username), psQuote(user.FullName)))
		if user.Role == "admin" || user.Role == "primary" {
			windows.WriteString(fmt.Sprintf("Add-LocalGroupMember -Group 'Administrators' -Member %s -ErrorAction SilentlyContinue\n", psQuote(user.Username)))
		}
		for _, group := range user.Groups {
			windows.WriteString(fmt.Sprintf("if (-not (Get-LocalGroup -Name %s -ErrorAction SilentlyContinue)) { New-LocalGroup -Name %s }; Add-LocalGroupMember -Group %s -Member %s -ErrorAction SilentlyContinue\n", psQuote(group), psQuote(group), psQuote(group), psQuote(user.Username)))
		}
	}
	return []ProjectScript{{Name: "Initialize Linux users", Platform: "linux", Content: linux.String()}, {Name: "Initialize Windows users", Platform: "windows", Content: windows.String()}}
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }
func psQuote(value string) string    { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func renderReadmeFragment(project Project) string {
	var output strings.Builder
	output.WriteString(sanitizeReadmeHTML(project.Readme.HTML))
	admins, users := []ImageUser{}, []ImageUser{}
	for _, user := range project.Users {
		if !user.AddToReadme {
			continue
		}
		if user.Role == "admin" || user.Role == "primary" {
			admins = append(admins, user)
		} else {
			users = append(users, user)
		}
	}
	if len(admins)+len(users) > 0 {
		output.WriteString("<h2>Authorized Administrators and Users</h2>")
		if len(admins) > 0 {
			output.WriteString("<p><strong>Authorized Administrators:</strong></p><ul>")
			for _, user := range admins {
				output.WriteString("<li>" + html.EscapeString(user.Username) + "</li>")
			}
			output.WriteString("</ul>")
		}
		if len(users) > 0 {
			output.WriteString("<p><strong>Authorized Users:</strong></p><ul>")
			for _, user := range users {
				output.WriteString("<li>" + html.EscapeString(user.Username) + "</li>")
			}
			output.WriteString("</ul>")
		}
	}
	return output.String()
}

func sanitizeReadmeHTML(input string) string {
	result := input
	for _, element := range []string{"script", "iframe", "object", "embed", "style"} {
		block := regexp.MustCompile(`(?is)<` + element + `[^>]*>.*?</` + element + `\s*>`)
		result = block.ReplaceAllString(result, "")
		standalone := regexp.MustCompile(`(?is)<` + element + `[^>]*/?\s*>`)
		result = standalone.ReplaceAllString(result, "")
	}
	events := regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	result = events.ReplaceAllString(result, "")
	javascriptURL := regexp.MustCompile(`(?i)(href|src)\s*=\s*("|')\s*javascript:[^"']*("|')`)
	result = javascriptURL.ReplaceAllString(result, `$1="#"`)
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
