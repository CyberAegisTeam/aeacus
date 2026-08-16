package main

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const projectFormatVersion = 1

type Project struct {
	FormatVersion int                 `json:"formatVersion"`
	StudioVersion string              `json:"studioVersion"`
	Name          string              `json:"name"`
	Config        ScoringConfig       `json:"config"`
	Readme        ReadmeDocument      `json:"readme"`
	Forensics     []ForensicQuestion  `json:"forensics"`
	Users         []ImageUser         `json:"users"`
	Scripts       []ProjectScript     `json:"scripts"`
	Audio         ProjectAudio        `json:"audio"`
	Progress      map[string]Progress `json:"progress"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

type ScoringConfig struct {
	Timezone                string  `toml:"timezone,omitempty" json:"timezone"`
	Name                    string  `toml:"name,omitempty" json:"name"`
	Title                   string  `toml:"title,omitempty" json:"title"`
	OS                      string  `toml:"os,omitempty" json:"os"`
	User                    string  `toml:"user,omitempty" json:"user"`
	Remote                  string  `toml:"remote,omitempty" json:"remote"`
	Password                string  `toml:"password,omitempty" json:"password"`
	Local                   bool    `toml:"local" json:"local"`
	EndDate                 string  `toml:"enddate,omitempty" json:"-"`
	Version                 string  `toml:"version,omitempty" json:"version"`
	DisableRemoteEncryption bool    `toml:"DisableRemoteEncryption,omitempty" json:"disableRemoteEncryption"`
	Checks                  []Check `toml:"check" json:"checks"`
}

type Check struct {
	ID           string      `toml:"id,omitempty" json:"id"`
	Category     string      `toml:"category,omitempty" json:"category"`
	Message      string      `toml:"message,omitempty" json:"message"`
	Hint         string      `toml:"hint,omitempty" json:"hint"`
	Points       int         `toml:"points" json:"points"`
	Pass         []Condition `toml:"pass,omitempty" json:"pass"`
	PassOverride []Condition `toml:"passoverride,omitempty" json:"passOverride"`
	Fail         []Condition `toml:"fail,omitempty" json:"fail"`
}

type Condition struct {
	Hint  string `toml:"hint,omitempty" json:"hint"`
	Type  string `toml:"type" json:"type"`
	Path  string `toml:"path,omitempty" json:"path"`
	Cmd   string `toml:"cmd,omitempty" json:"cmd"`
	User  string `toml:"user,omitempty" json:"user"`
	Group string `toml:"group,omitempty" json:"group"`
	Name  string `toml:"name,omitempty" json:"name"`
	Key   string `toml:"key,omitempty" json:"key"`
	Value string `toml:"value,omitempty" json:"value"`
	After string `toml:"after,omitempty" json:"after"`
}

type ReadmeDocument struct {
	HTML string `json:"html"`
}

type ForensicQuestion struct {
	Number int            `json:"number"`
	Title  string         `json:"title"`
	Intro  string         `json:"intro"`
	Parts  []ForensicPart `json:"parts"`
}

type ForensicPart struct {
	Prompt             string   `json:"prompt"`
	Answers            []string `json:"answers"`
	CaseSensitive      bool     `json:"caseSensitive"`
	FlexibleWhitespace bool     `json:"flexibleWhitespace"`
}

type ImageUser struct {
	Username       string   `json:"username"`
	FullName       string   `json:"fullName"`
	Role           string   `json:"role"`
	Password       string   `json:"password"`
	Shell          string   `json:"shell"`
	Groups         []string `json:"groups"`
	AddToReadme    bool     `json:"addToReadme"`
	RemovalPenalty bool     `json:"removalPenalty"`
	PasswordCheck  bool     `json:"passwordCheck"`
	GroupChecks    bool     `json:"groupChecks"`
}

type ProjectScript struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Content  string `json:"content"`
}

type ProjectAudio struct {
	GainName string `json:"gainName"`
	GainData string `json:"gainData"`
	LossName string `json:"lossName"`
	LossData string `json:"lossData"`
}

type Progress struct {
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
	SeenPassing  bool      `json:"seenPassing"`
	SeenFailing  bool      `json:"seenFailing"`
	LastResult   string    `json:"lastResult"`
	LastObserved time.Time `json:"lastObserved"`
}

type ValidationIssue struct {
	Level   string `json:"level"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

func newProject() Project {
	project := Project{
		FormatVersion: projectFormatVersion,
		StudioVersion: studioVersion,
		Name:          "Untitled Aeacus Image",
		Config: ScoringConfig{
			Timezone: "America/Los_Angeles",
			Remote:   "https://scoring.cyberaegis.tech",
			Local:    false,
			Version:  studioVersion,
		},
		Readme:    ReadmeDocument{HTML: `<h2>Critical Services</h2><ul><li>Add critical services here</li></ul><h2>Authorized Users</h2><p>Add authorized users here.</p>`},
		Progress:  map[string]Progress{},
		UpdatedAt: time.Now(),
	}
	normalizeProject(&project)
	return project
}

// normalizeProject guarantees that every collection sent to the desktop UI is
// an empty array/object rather than JSON null. Legacy TOML files omit optional
// condition groups, which is valid and must never make the editor crash.
func normalizeProject(project *Project) {
	if project.Config.Checks == nil {
		project.Config.Checks = []Check{}
	}
	for index := range project.Config.Checks {
		check := &project.Config.Checks[index]
		if check.Pass == nil {
			check.Pass = []Condition{}
		}
		if check.PassOverride == nil {
			check.PassOverride = []Condition{}
		}
		if check.Fail == nil {
			check.Fail = []Condition{}
		}
	}
	if project.Forensics == nil {
		project.Forensics = []ForensicQuestion{}
	}
	for questionIndex := range project.Forensics {
		question := &project.Forensics[questionIndex]
		if question.Parts == nil {
			question.Parts = []ForensicPart{}
		}
		for partIndex := range question.Parts {
			if question.Parts[partIndex].Answers == nil {
				question.Parts[partIndex].Answers = []string{}
			}
		}
	}
	if project.Users == nil {
		project.Users = []ImageUser{}
	}
	for index := range project.Users {
		if project.Users[index].Groups == nil {
			project.Users[index].Groups = []string{}
		}
	}
	if project.Scripts == nil {
		project.Scripts = []ProjectScript{}
	}
	if project.Progress == nil {
		project.Progress = map[string]Progress{}
	}
}

func parseScoringConfig(content string) (ScoringConfig, error) {
	var config ScoringConfig
	if strings.TrimSpace(content) == "" {
		return config, errors.New("scoring.conf is empty")
	}
	metadata, err := toml.Decode(content, &config)
	if err != nil {
		return config, err
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		return config, fmt.Errorf("unsupported or misspelled field: %s", undecoded[0].String())
	}
	ensureCheckIDs(&config)
	applyCommentCategories(content, &config)
	project := Project{Config: config}
	normalizeProject(&project)
	config = project.Config
	return config, nil
}

func applyCommentCategories(content string, config *ScoringConfig) {
	category, checkIndex := "General", 0
	heading := regexp.MustCompile(`^\s*#{3,}\s*(.*?)\s*$`)
	for _, line := range strings.Split(content, "\n") {
		if match := heading.FindStringSubmatch(line); len(match) == 2 && strings.TrimSpace(match[1]) != "" {
			category = strings.TrimSpace(match[1])
			continue
		}
		if strings.TrimSpace(line) == "[[check]]" && checkIndex < len(config.Checks) {
			config.Checks[checkIndex].Category = category
			checkIndex++
		}
	}
}

func renderScoringConfig(config ScoringConfig) (string, error) {
	copyConfig := config
	copyConfig.EndDate = ""
	var output bytes.Buffer
	if err := toml.NewEncoder(&output).Encode(copyConfig); err != nil {
		return "", err
	}
	return output.String(), nil
}

func ensureCheckIDs(config *ScoringConfig) {
	used := map[string]bool{}
	for i := range config.Checks {
		base := config.Checks[i].ID
		if base == "" {
			base = slug(config.Checks[i].Message)
		}
		if base == "" {
			base = fmt.Sprintf("check-%d", i+1)
		}
		candidate := base
		for suffix := 2; used[candidate]; suffix++ {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		config.Checks[i].ID = candidate
		used[candidate] = true
	}
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func validateProject(project Project) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(level, path, message string) { issues = append(issues, ValidationIssue{level, path, message}) }
	config := project.Config
	if strings.TrimSpace(config.Name) == "" {
		add("error", "config.name", "Image name is required")
	}
	if strings.TrimSpace(config.Title) == "" {
		add("error", "config.title", "Round title is required")
	}
	if strings.TrimSpace(config.OS) == "" {
		add("error", "config.os", "Operating system is required")
	}
	if strings.TrimSpace(config.User) == "" {
		add("error", "config.user", "Main user is required")
	} else if !regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(config.User) {
		add("error", "config.user", "Main user contains unsafe path characters")
	}
	if config.Version != studioVersion {
		add("warning", "config.version", "Set the Aeacus configuration version to "+studioVersion)
	}
	if config.Remote != "" && strings.HasSuffix(config.Remote, "/") {
		add("error", "config.remote", "Remote URL must not end in a slash")
	}
	if config.Remote != "" && config.Password == "" && !config.DisableRemoteEncryption {
		add("error", "config.password", "Remote password is required")
	}
	if len(config.Checks) == 0 {
		add("error", "config.checks", "At least one vulnerability is required")
	}
	total := 0
	ids := map[string]bool{}
	for i, check := range config.Checks {
		path := fmt.Sprintf("config.checks[%d]", i)
		if check.ID == "" {
			add("warning", path+".id", "Stable check ID will be generated")
		}
		if ids[check.ID] && check.ID != "" {
			add("error", path+".id", "Check ID is duplicated")
		}
		ids[check.ID] = true
		if strings.TrimSpace(check.Message) == "" {
			add("warning", path+".message", "A participant-facing message is recommended")
		}
		if len(check.Pass) == 0 && len(check.PassOverride) == 0 {
			add("error", path, "Check has no way to pass")
		}
		if check.Points > 0 {
			total += check.Points
		}
		validateConditions := func(kind string, conditions []Condition) {
			for j, condition := range conditions {
				conditionPath := fmt.Sprintf("%s.%s[%d]", path, kind, j)
				if strings.TrimSpace(condition.Type) == "" {
					add("error", conditionPath+".type", "Condition type is required")
				}
				for _, field := range requiredConditionFields(condition.Type) {
					if conditionField(condition, field) == "" {
						add("error", conditionPath+"."+strings.ToLower(field), field+" is required for "+condition.Type)
					}
				}
				if strings.Contains(condition.Type, "Regex") && condition.Value != "" {
					if _, err := regexp.Compile(condition.Value); err != nil {
						add("error", conditionPath+".value", "Invalid regular expression: "+err.Error())
					}
				}
				if strings.Contains(condition.Value, "REPLACE_") || strings.Contains(condition.After, "REPLACE_") {
					add("warning", conditionPath, "Condition still contains an author placeholder")
				}
			}
		}
		validateConditions("pass", check.Pass)
		validateConditions("passOverride", check.PassOverride)
		validateConditions("fail", check.Fail)
	}
	for i, user := range project.Users {
		if user.Username == "" || !regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(user.Username) {
			add("error", fmt.Sprintf("users[%d].username", i), "Username is empty or contains unsafe characters")
		}
	}
	if total != 100 {
		add("warning", "config.checks", fmt.Sprintf("Explicit positive points total %d, not 100", total))
	}
	if strings.Contains(config.Password, "REPLACE") {
		add("warning", "config.password", "Password still contains a placeholder")
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Level == "error" && issues[j].Level != "error" })
	return issues
}

func requiredConditionFields(conditionType string) []string {
	base := strings.TrimSuffix(conditionType, "Not")
	base = strings.TrimSuffix(base, "Regex")
	switch base {
	case "FileContains", "DirContains":
		return []string{"Path", "Value"}
	case "FileEquals":
		return []string{"Path", "Value"}
	case "FirewallDefaultBehavior":
		return []string{"Name", "Key", "Value"}
	case "PathExists":
		return []string{"Path"}
	case "Command":
		return []string{"Cmd"}
	case "CommandContains", "CommandOutput":
		return []string{"Cmd", "Value"}
	case "UserExists":
		return []string{"User"}
	case "UserInGroup":
		return []string{"User", "Group"}
	case "ProgramInstalled":
		return []string{"Name"}
	case "ProgramVersion":
		return []string{"Name", "Value"}
	case "ServiceUp", "ServiceStartup", "ScheduledTaskExists", "ShareExists", "WindowsFeature":
		return []string{"Name"}
	case "PermissionIs":
		return []string{"Path", "Value"}
	case "FileOwner":
		return []string{"Path", "Name"}
	case "PasswordChanged":
		return []string{"User"}
	case "SecurityPolicy", "RegistryKey":
		return []string{"Key", "Value"}
	case "RegistryKeyExists":
		return []string{"Key"}
	case "UserDetail":
		return []string{"User", "Key", "Value"}
	case "UserRights":
		return []string{"User", "Value"}
	}
	return nil
}

func conditionField(condition Condition, field string) string {
	switch field {
	case "Path":
		return condition.Path
	case "Value":
		return condition.Value
	case "Cmd":
		return condition.Cmd
	case "User":
		return condition.User
	case "Group":
		return condition.Group
	case "Name":
		return condition.Name
	case "Key":
		return condition.Key
	case "After":
		return condition.After
	}
	return ""
}
