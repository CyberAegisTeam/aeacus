package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestScoringConfigRoundTrip(t *testing.T) {
	input := `timezone = "America/Los_Angeles"
name = "Linux_PR6"
title = "Practice Round 6"
os = "Linux Mint 22"
user = "red"
remote = "https://scoring.example"
password = "secret"
local = false
version = "3.0.0"

##### USERS / GROUPS
[[check]]
message = "Unauthorized user removed"
points = 5
  [[check.pass]]
  type = "UserExistsNot"
  user = "baduser"

##### SERVICES
[[check]]
message = "Service enabled"
points = 5
  [[check.pass]]
  type = "ServiceUp"
  name = "sshd"
`
	config, err := parseScoringConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Checks) != 2 {
		t.Fatalf("got %d checks", len(config.Checks))
	}
	if config.Checks[0].Category != "USERS / GROUPS" || config.Checks[1].Category != "SERVICES" {
		t.Fatalf("categories not imported: %#v", config.Checks)
	}
	output, err := renderScoringConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := parseScoringConfig(output)
	if err != nil {
		t.Fatal(err)
	}
	if reparsed.Checks[0].Pass[0].User != "baduser" {
		t.Fatalf("condition lost: %s", output)
	}
}

func TestIntegerRegex(t *testing.T) {
	tests := []struct {
		operator           string
		minimum, maximum   int
		accepted, rejected []string
	}{
		{">", 1, 0, []string{"2", "10", "1002", "99999"}, []string{"0", "1"}},
		{">=", 25, 0, []string{"25", "26", "10000"}, []string{"0", "24"}},
		{"<", 5, 0, []string{"0", "4"}, []string{"5", "50"}},
		{"between", 12, 19, []string{"12", "15", "19"}, []string{"11", "20", "120"}},
	}
	for _, test := range tests {
		pattern, _, err := integerRegex(test.operator, test.minimum, test.maximum)
		if err != nil {
			t.Fatal(err)
		}
		expression, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			t.Fatalf("compile %s: %v", pattern, err)
		}
		for _, value := range test.accepted {
			if !expression.MatchString(value) {
				t.Errorf("%s %d should accept %s: %s", test.operator, test.minimum, value, pattern)
			}
		}
		for _, value := range test.rejected {
			if expression.MatchString(value) {
				t.Errorf("%s %d should reject %s: %s", test.operator, test.minimum, value, pattern)
			}
		}
	}
}

func TestSettingRegexSeparators(t *testing.T) {
	for separator, sample := range map[string]string{"equals": "PermitRootLogin = no", "whitespace": "PermitRootLogin no", "colon": "PermitRootLogin: no"} {
		result, err := buildRegex(RegexRequest{Mode: "setting", Key: "PermitRootLogin", Separator: separator, Values: []string{"no"}, FlexibleWhitespace: true, WholeLine: true})
		if err != nil {
			t.Fatal(err)
		}
		pattern := strings.TrimPrefix(result.Pattern, "(?i)")
		pattern = strings.ReplaceAll(pattern, "[[:space:]]", `\s`)
		if matched, err := regexp.MatchString(pattern, sample); err != nil || !matched {
			t.Fatalf("separator %s pattern %q did not match %q: %v", separator, result.Pattern, sample, err)
		}
	}
}

func TestGeneratedContent(t *testing.T) {
	project := newProject()
	project.Config.Name, project.Config.Title, project.Config.OS, project.Config.User, project.Config.Password = "Linux_PR6", "PR6", "Linux Mint", "red", "secret"
	project.Users = []ImageUser{{Username: "student", Role: "admin", Password: "temporary", Groups: []string{"pigs"}, AddToReadme: true, RemovalPenalty: true, GroupChecks: true}}
	project.Forensics = []ForensicQuestion{{Title: "FQ1", Parts: []ForensicPart{{Prompt: "Find the value", Answers: []string{"answer one"}, FlexibleWhitespace: true}}}}
	synchronizeGeneratedChecks(&project)
	if len(project.Config.Checks) != 2 {
		t.Fatalf("expected only removal and group checks; FQs must remain standalone; got %d", len(project.Config.Checks))
	}
	for _, check := range project.Config.Checks {
		if strings.HasPrefix(check.ID, "studio-fq-") {
			t.Fatal("forensic question was added to scoring checks")
		}
	}
	files := generateForensicFiles(&project)
	if !strings.Contains(files["FQ1.txt"], "ANSWER:") {
		t.Fatal("missing answer marker")
	}
	scripts := generateSetupScripts(project)
	if !strings.Contains(scripts[0].Content, "useradd -m") || !strings.Contains(scripts[1].Content, "New-LocalUser") {
		t.Fatal("user setup scripts incomplete")
	}
	if !strings.Contains(renderReadmeFragment(project), "student") {
		t.Fatal("authorized user not included in README")
	}
}

func TestValidation(t *testing.T) {
	project := newProject()
	project.Config.Checks = []Check{{Message: "Broken regex", Points: 100, Pass: []Condition{{Type: "FileContainsRegex", Path: "/tmp/a", Value: "["}}}}
	issues := validateProject(project)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "Invalid regular expression") {
			found = true
		}
	}
	if !found {
		t.Fatal("invalid regex was not detected")
	}
}

func TestLegacyConfigCollectionsEncodeAsArrays(t *testing.T) {
	config, err := parseScoringConfig(`
name = "Linux_PR6"
title = "Linux Practice Round 6 [2026]"
os = "Linux Mint 22"
user = "red"

[[check]]
message = "Removed unauthorized file"
points = 5
  [[check.pass]]
  type = "PathExistsNot"
  path = "/tmp/unauthorized.mp3"
`)
	if err != nil {
		t.Fatal(err)
	}
	project := newProject()
	project.Config = config
	normalizeProject(&project)
	encoded, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, unexpected := range []string{`"passOverride":null`, `"fail":null`, `"forensics":null`, `"users":null`} {
		if strings.Contains(string(encoded), unexpected) {
			t.Fatalf("legacy project contains a null collection %s: %s", unexpected, encoded)
		}
	}
}

func TestEndDateIsNotExportedByStudio(t *testing.T) {
	project := newProject()
	project.Config.EndDate = "2026/08/16 16:00:00 PDT"
	output, err := renderScoringConfig(project.Config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(output), "enddate") {
		t.Fatalf("Studio exported removed end-date option: %s", output)
	}
}

func TestReadmeSanitization(t *testing.T) {
	input := `<h2 onclick="bad()">Scenario</h2><script>alert(1)</script><a href="javascript:bad()">link</a>`
	output := sanitizeReadmeHTML(input)
	if strings.Contains(strings.ToLower(output), "script") || strings.Contains(strings.ToLower(output), "onclick") || strings.Contains(strings.ToLower(output), "javascript:") {
		t.Fatalf("unsafe README HTML survived: %s", output)
	}
}

func TestAuthoringExportsStaySeparate(t *testing.T) {
	project := newProject()
	project.Config.OS = "Windows 11"
	project.Config.Checks = []Check{{ID: "manual", Category: "Legacy", Hint: "unused", Message: "Manual check", Points: 100, Pass: []Condition{{Type: "PathExists", Path: `C:\safe`, Hint: "unused"}}}, {ID: "studio-fq-1-1", Message: "Old generated FQ", Points: 5}}
	project.Forensics = []ForensicQuestion{{Title: "Investigation", Parts: []ForensicPart{{Prompt: "What happened?", Answers: []string{"legacy answer"}}}}}
	synchronizeGeneratedChecks(&project)
	if len(project.Config.Checks) != 1 || project.Config.Checks[0].ID != "manual" {
		t.Fatalf("FQ checks were not removed: %#v", project.Config.Checks)
	}
	config, err := renderScoringConfig(project.Config)
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"category", "hint", "Old generated FQ", "legacy answer"} {
		if strings.Contains(config, unwanted) {
			t.Fatalf("scoring.conf contains %q: %s", unwanted, config)
		}
	}
	files := generateForensicFiles(&project)
	if !strings.Contains(files["FQ1.txt"], "What happened?") || strings.Contains(files["FQ1.txt"], "legacy answer") {
		t.Fatalf("unexpected FQ file: %s", files["FQ1.txt"])
	}
	readme := renderReadmeHTML(project)
	for _, heading := range []string{"Unique Identifier", "Forensics Questions", "Competition Scenario", "Competition Guidelines", "currently does not use any centralized maintenance or polling tools", "time zone of this image is set to UTC", "Copyright Never"} {
		if !strings.Contains(readme, heading) {
			t.Fatalf("README missing %s", heading)
		}
	}
}

func TestGuidedUserScripts(t *testing.T) {
	project := newProject()
	project.Users = []ImageUser{{Username: "normal", Role: "user"}, {Username: "admin", Role: "admin", Password: "SecurePass"}}
	scripts := generateSetupScripts(project)
	if !strings.Contains(scripts[0].Content, "'normal':'password'") || !strings.Contains(scripts[0].Content, "usermod -aG sudo 'admin'") {
		t.Fatalf("Linux wizard script is incomplete: %s", scripts[0].Content)
	}
	if !strings.Contains(scripts[1].Content, "-NoPassword") || !strings.Contains(scripts[1].Content, `C:\Users`) || !strings.Contains(scripts[1].Content, "'Desktop'") {
		t.Fatalf("Windows wizard script is incomplete: %s", scripts[1].Content)
	}
}
