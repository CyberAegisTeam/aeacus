package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const studioVersion = "3.0.0"

var defaultMode = "personal"

//go:embed web/*
var webFiles embed.FS

type studioServer struct {
	mu          sync.RWMutex
	project     Project
	mode        string
	token       string
	projectPath string
	recent      []string
	targetDir   string
	ctx         context.Context
}

type apiError struct {
	Error string `json:"error"`
}

func main() {
	mode := flag.String("mode", defaultMode, "personal or development")
	projectPath := flag.String("project", "", "Aeacus project to open")
	targetDir := flag.String("target", defaultTargetDir(), "Aeacus image directory (Development Studio)")
	flag.Parse()
	if *mode != "personal" && *mode != "development" {
		log.Fatal("mode must be personal or development")
	}
	server := &studioServer{mode: *mode, token: randomToken(), projectPath: *projectPath, targetDir: *targetDir, project: newProject()}
	server.recent = loadRecentProjects(*mode)
	if *projectPath != "" {
		if err := server.loadProject(*projectPath); err != nil {
			log.Fatalf("open project: %v", err)
		}
	} else if configDir, err := os.UserConfigDir(); err == nil {
		server.projectPath = filepath.Join(configDir, "Aeacus Studio", *mode+"-recovery.aeacus")
		if _, err := os.Stat(server.projectPath); err == nil {
			if err := server.loadProject(server.projectPath); err != nil {
				log.Printf("Ignoring invalid recovery project: %v", err)
				server.project = newProject()
			}
		}
	}
	title := "Aeacus Studio Personal"
	if *mode == "development" {
		title = "Aeacus Studio Development"
	}
	if err := wails.Run(&options.App{
		Title:            title,
		Width:            1480,
		Height:           940,
		MinWidth:         1100,
		MinHeight:        720,
		WindowStartState: options.Maximised,
		BackgroundColour: options.NewRGB(8, 15, 28),
		AssetServer:      &assetserver.Options{Handler: server.routes()},
		OnStartup: func(ctx context.Context) {
			server.mu.Lock()
			server.ctx = ctx
			server.mu.Unlock()
		},
		EnableDefaultContextMenu: true,
		Mac:                      &mac.Options{TitleBar: mac.TitleBarDefault(), Appearance: mac.NSAppearanceNameDarkAqua},
		Windows:                  &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false},
	}); err != nil {
		log.Fatal(err)
	}
}

func (s *studioServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveWeb)
	mux.HandleFunc("/api/state", s.api(s.handleState))
	mux.HandleFunc("/api/project", s.api(s.handleProject))
	mux.HandleFunc("/api/project/new", s.api(s.personalOnly(s.handleNewProject)))
	mux.HandleFunc("/api/project/create-workspace", s.api(s.personalOnly(s.handleCreateWorkspace)))
	mux.HandleFunc("/api/project/open-workspace", s.api(s.personalOnly(s.handleOpenWorkspace)))
	mux.HandleFunc("/api/projects/recent", s.api(s.personalOnly(s.handleRecentProjects)))
	mux.HandleFunc("/api/project/switch", s.api(s.personalOnly(s.handleSwitchProject)))
	mux.HandleFunc("/api/project/download", s.api(s.personalOnly(s.handleProjectDownload)))
	mux.HandleFunc("/api/project/import", s.api(s.handleProjectImport))
	mux.HandleFunc("/api/config/import", s.api(s.handleConfigImport))
	mux.HandleFunc("/api/config/download", s.api(s.handleConfigDownload))
	mux.HandleFunc("/api/readme/download", s.api(s.handleReadmeDownload))
	mux.HandleFunc("/api/readme/preview", s.api(s.handleReadmePreview))
	mux.HandleFunc("/api/forensics/download", s.api(s.handleForensicsDownload))
	mux.HandleFunc("/api/scripts/generate", s.api(s.handleGenerateScripts))
	mux.HandleFunc("/api/regex", s.api(s.handleRegex))
	mux.HandleFunc("/api/scoring-data", s.api(s.handleScoringData))
	mux.HandleFunc("/api/save", s.api(s.handleDesktopSave))
	mux.HandleFunc("/api/development/initialize", s.api(s.developmentOnly(s.handleInitialize)))
	mux.HandleFunc("/api/development/install", s.api(s.developmentOnly(s.handleInstallProject)))
	mux.HandleFunc("/api/development/evaluate", s.api(s.developmentOnly(s.handleEvaluate)))
	mux.HandleFunc("/api/development/run-script", s.api(s.developmentOnly(s.handleRunScript)))
	mux.HandleFunc("/api/development/release", s.api(s.developmentOnly(s.handleRelease)))
	return mux
}

func (s *studioServer) handleNewProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	project := newProject()
	s.mu.Lock()
	s.project = project
	path := s.projectPath
	s.mu.Unlock()
	if path != "" {
		if err := s.saveProject(path); err != nil {
			writeJSON(w, 500, apiError{err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"project": project, "issues": validateProject(project)})
}

func (s *studioServer) handleDesktopSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Kind string `json:"kind"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.mu.RLock()
	project := s.project
	ctx := s.ctx
	s.mu.RUnlock()
	var name, mimeType string
	var data []byte
	var err error
	switch request.Kind {
	case "project":
		name, mimeType = slug(project.Name)+".aeacus", "application/json"
		data, err = json.MarshalIndent(project, "", "  ")
	case "config":
		name, mimeType = "scoring.conf", "text/plain"
		var content string
		content, err = renderScoringConfig(project.Config)
		data = []byte(content)
	case "readme":
		name, mimeType = "ReadMe.conf", "text/html"
		data = []byte(renderReadmeFragment(project))
	case "readmeHTML":
		name, mimeType = "ReadMe.html", "text/html"
		data = []byte(renderReadmeHTML(project))
	case "scoringData":
		for _, issue := range validateProject(project) {
			if issue.Level == "error" {
				writeJSON(w, 400, apiError{"configuration has validation errors"})
				return
			}
		}
		name, mimeType = scoringDataFilename(project.Config), "application/octet-stream"
		var content, encrypted string
		content, err = renderScoringConfig(project.Config)
		if err == nil {
			encrypted, err = encryptScoringData(content)
			data = []byte(encrypted)
		}
	default:
		writeJSON(w, 400, apiError{"unknown export type"})
		return
	}
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if name == ".aeacus" {
		name = "aeacus-image.aeacus"
	}
	if ctx == nil {
		writeJSON(w, 503, apiError{"desktop window is not ready"})
		return
	}
	path, err := wailsRuntime.SaveFileDialog(ctx, wailsRuntime.SaveDialogOptions{DefaultFilename: name, Title: "Save " + name})
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if path == "" {
		writeJSON(w, 200, map[string]any{"cancelled": true})
		return
	}
	if err := atomicWrite(path, data, 0600); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"path": path, "type": mimeType})
}

func (s *studioServer) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "Untitled Aeacus Image"
	}
	directoryName := slug(name)
	if directoryName == "" {
		directoryName = "aeacus-project"
	}
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		writeJSON(w, 503, apiError{"desktop window is not ready"})
		return
	}
	parent, err := wailsRuntime.OpenDirectoryDialog(ctx, wailsRuntime.OpenDialogOptions{Title: "Choose where to create the Aeacus project"})
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if parent == "" {
		writeJSON(w, 200, map[string]any{"cancelled": true})
		return
	}
	directory := filepath.Join(parent, directoryName)
	path := filepath.Join(directory, "project.aeacus")
	if _, err := os.Stat(path); err == nil {
		writeJSON(w, 409, apiError{"A project with this name already exists in that location"})
		return
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	project := newProject()
	project.Name = name
	s.mu.Lock()
	s.project, s.projectPath = project, path
	s.addRecentLocked(path)
	s.mu.Unlock()
	if err := s.saveProject(path); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"project": project, "issues": validateProject(project), "path": path, "recent": s.recent})
}

func (s *studioServer) handleOpenWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		writeJSON(w, 503, apiError{"desktop window is not ready"})
		return
	}
	directory, err := wailsRuntime.OpenDirectoryDialog(ctx, wailsRuntime.OpenDialogOptions{Title: "Open an Aeacus project folder"})
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if directory == "" {
		writeJSON(w, 200, map[string]any{"cancelled": true})
		return
	}
	path := filepath.Join(directory, "project.aeacus")
	if _, err := os.Stat(path); err != nil {
		writeJSON(w, 400, apiError{"This folder does not contain project.aeacus"})
		return
	}
	if err := s.openProjectPath(path); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.writeProjectState(w)
}

func (s *studioServer) handleSwitchProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	if err := s.openProjectPath(request.Path); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.writeProjectState(w)
}

func (s *studioServer) handleRecentProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"current": s.projectPath, "recent": s.recent})
}
func (s *studioServer) writeProjectState(w http.ResponseWriter) {
	s.mu.RLock()
	project, path, recent := s.project, s.projectPath, append([]string(nil), s.recent...)
	s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"project": project, "issues": validateProject(project), "path": path, "recent": recent})
}
func (s *studioServer) openProjectPath(path string) error {
	if err := s.loadProject(path); err != nil {
		return err
	}
	s.mu.Lock()
	s.projectPath = path
	s.addRecentLocked(path)
	s.mu.Unlock()
	return nil
}
func (s *studioServer) addRecentLocked(path string) {
	next := []string{path}
	for _, item := range s.recent {
		if item != path {
			if _, err := os.Stat(item); err == nil {
				next = append(next, item)
			}
		}
		if len(next) >= 8 {
			break
		}
	}
	s.recent = next
	saveRecentProjects(s.mode, next)
}
func recentProjectsPath(mode string) string {
	directory, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(directory, "Aeacus Studio", mode+"-recent.json")
}
func loadRecentProjects(mode string) []string {
	path := recentProjectsPath(mode)
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	var items []string
	if json.Unmarshal(data, &items) != nil {
		return []string{}
	}
	return items
}
func saveRecentProjects(mode string, items []string) {
	path := recentProjectsPath(mode)
	if path == "" {
		return
	}
	data, _ := json.Marshal(items)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = atomicWrite(path, data, 0600)
}

func (s *studioServer) api(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Header.Get("X-Aeacus-Token") != s.token && r.URL.Query().Get("token") != s.token {
			writeJSON(w, http.StatusForbidden, apiError{"invalid Studio session token"})
			return
		}
		next(w, r)
	}
}

func (s *studioServer) developmentOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.mode != "development" {
			writeJSON(w, http.StatusForbidden, apiError{"Development Studio only"})
			return
		}
		next(w, r)
	}
}

func (s *studioServer) personalOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.mode != "personal" {
			writeJSON(w, http.StatusForbidden, apiError{"Personal Studio only"})
			return
		}
		next(w, r)
	}
}

func (s *studioServer) serveWeb(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	data, err := webFiles.ReadFile("web/" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if path == "index.html" {
		data = []byte(strings.ReplaceAll(string(data), "__STUDIO_BOOTSTRAP__", fmt.Sprintf(`{"token":%q,"mode":%q,"version":%q}`, s.token, s.mode, studioVersion)))
	}
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (s *studioServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"project": s.project, "issues": validateProject(s.project), "mode": s.mode, "targetDir": s.targetDir, "os": runtime.GOOS, "projectPath": s.projectPath, "recent": s.recent})
}

func (s *studioServer) handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	var project Project
	if err := decodeJSON(r, &project); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	if project.FormatVersion == 0 {
		project.FormatVersion = projectFormatVersion
	}
	if project.FormatVersion != projectFormatVersion {
		writeJSON(w, http.StatusBadRequest, apiError{"unsupported project format"})
		return
	}
	project.StudioVersion = studioVersion
	project.UpdatedAt = time.Now()
	ensureCheckIDs(&project.Config)
	synchronizeGeneratedChecks(&project)
	normalizeProject(&project)
	s.mu.Lock()
	s.project = project
	path := s.projectPath
	s.mu.Unlock()
	if path != "" {
		if err := s.saveProject(path); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": project, "issues": validateProject(project)})
}

func (s *studioServer) handleProjectDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	data, err := json.MarshalIndent(s.project, "", "  ")
	name := slug(s.project.Name)
	s.mu.RUnlock()
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if name == "" {
		name = "aeacus-image"
	}
	download(w, name+".aeacus", "application/json", data)
}

func (s *studioServer) handleProjectImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var project Project
	if err := decodeJSON(r, &project); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	if project.FormatVersion != projectFormatVersion {
		writeJSON(w, 400, apiError{"unsupported project format"})
		return
	}
	ensureCheckIDs(&project.Config)
	normalizeProject(&project)
	s.mu.Lock()
	s.project = project
	path := s.projectPath
	s.mu.Unlock()
	if path != "" {
		if err := s.saveProject(path); err != nil {
			writeJSON(w, 500, apiError{err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"project": project, "issues": validateProject(project)})
}

func (s *studioServer) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	config, err := parseScoringConfig(input.Content)
	if err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.mu.Lock()
	s.project.Config = config
	s.project.Name = config.Title
	s.project.UpdatedAt = time.Now()
	normalizeProject(&s.project)
	project := s.project
	path := s.projectPath
	s.mu.Unlock()
	if path != "" {
		if err := s.saveProject(path); err != nil {
			writeJSON(w, 500, apiError{err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"project": project, "issues": validateProject(project)})
}

func (s *studioServer) handleConfigDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	content, err := renderScoringConfig(s.project.Config)
	s.mu.RUnlock()
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	download(w, "scoring.conf", "text/plain; charset=utf-8", []byte(content))
}

func (s *studioServer) handleReadmeDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	content := renderReadmeFragment(s.project)
	s.mu.RUnlock()
	download(w, "ReadMe.conf", "text/html; charset=utf-8", []byte(content))
}

func (s *studioServer) handleReadmePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	content := renderReadmeHTML(s.project)
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, content)
}

func (s *studioServer) handleForensicsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	files := generateForensicFiles(&s.project)
	s.mu.RUnlock()
	writeJSON(w, 200, files)
}

func (s *studioServer) handleGenerateScripts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	s.mu.Lock()
	s.project.Scripts = generateSetupScripts(s.project)
	scripts := s.project.Scripts
	s.mu.Unlock()
	writeJSON(w, 200, scripts)
}

func (s *studioServer) handleRegex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request RegexRequest
	if err := decodeJSON(r, &request); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	response, err := buildRegex(request)
	if err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	response.Matches, err = evaluateRegexSamples(response.Pattern, request.Samples)
	if err != nil { writeJSON(w, 400, apiError{err.Error()}); return }
	writeJSON(w, 200, response)
}

func (s *studioServer) handleScoringData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.RLock()
	config := s.project.Config
	project := s.project
	s.mu.RUnlock()
	for _, issue := range validateProject(project) {
		if issue.Level == "error" {
			writeJSON(w, 400, apiError{"configuration has validation errors"})
			return
		}
	}
	content, err := renderScoringConfig(config)
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	encrypted, err := encryptScoringData(content)
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	download(w, scoringDataFilename(config), "application/octet-stream", []byte(encrypted))
}

func (s *studioServer) handleInitialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Target string `json:"target"`
	}
	_ = decodeJSON(r, &request)
	if request.Target == "" {
		request.Target = s.targetDir
	}
	if err := validateTarget(request.Target); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	if err := os.MkdirAll(request.Target, 0755); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	source, _ := os.Executable()
	sourceDir := filepath.Dir(source)
	items := []string{"assets", "misc", runtimeBinaryName("aeacus"), runtimeBinaryName("phocus")}
	copied := []string{}
	for _, item := range items {
		from := filepath.Join(sourceDir, item)
		to := filepath.Join(request.Target, item)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := copyPath(from, to); err != nil {
			writeJSON(w, 500, apiError{fmt.Sprintf("copy %s: %v", item, err)})
			return
		}
		copied = append(copied, item)
	}
	s.targetDir = request.Target
	writeJSON(w, 200, map[string]any{"target": request.Target, "copied": copied, "message": "Image initialized"})
}

func (s *studioServer) handleInstallProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := validateTarget(s.targetDir); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.mu.Lock()
	synchronizeGeneratedChecks(&s.project)
	project := s.project
	s.mu.Unlock()
	for _, issue := range validateProject(project) {
		if issue.Level == "error" {
			writeJSON(w, 400, apiError{"project has validation errors; installation stopped"})
			return
		}
	}
	config, err := renderScoringConfig(project.Config)
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if err := atomicWrite(filepath.Join(s.targetDir, "scoring.conf"), []byte(config), 0600); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	if err := atomicWrite(filepath.Join(s.targetDir, "ReadMe.conf"), []byte(renderReadmeFragment(project)), 0644); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	home := homeForConfig(project.Config)
	desktop := filepath.Join(home, "Desktop")
	if err := os.MkdirAll(desktop, 0755); err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	for name, content := range generateForensicFiles(&project) {
		if err := atomicWrite(filepath.Join(desktop, name), []byte(content), 0644); err != nil {
			writeJSON(w, 500, apiError{err.Error()})
			return
		}
	}
	for _, script := range project.Scripts {
		ext := ".sh"
		if script.Platform == "windows" {
			ext = ".ps1"
		}
		if err := atomicWrite(filepath.Join(s.targetDir, slug(script.Name)+ext), []byte(script.Content), 0700); err != nil {
			writeJSON(w, 500, apiError{err.Error()})
			return
		}
	}
	if project.Audio.GainData != "" {
		if err := writeAudio(filepath.Join(s.targetDir, "assets", "wav", "gain.wav"), project.Audio.GainData); err != nil {
			writeJSON(w, 400, apiError{"gain sound: " + err.Error()})
			return
		}
	}
	if project.Audio.LossData != "" {
		if err := writeAudio(filepath.Join(s.targetDir, "assets", "wav", "alarm.wav"), project.Audio.LossData); err != nil {
			writeJSON(w, 400, apiError{"loss sound: " + err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"message": "Project installed", "target": s.targetDir, "forensics": len(project.Forensics)})
}

func (s *studioServer) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	binary := filepath.Join(s.targetDir, runtimeBinaryName("aeacus"))
	command := exec.Command(binary, "--dir", withTrailingSeparator(s.targetDir), "studio-evaluate")
	command.Dir = s.targetDir
	output, err := command.CombinedOutput()
	if err != nil {
		writeJSON(w, 500, apiError{strings.TrimSpace(string(output)) + ": " + err.Error()})
		return
	}
	var evaluation struct {
		Score   int `json:"score"`
		Results []struct {
			ID     string `json:"id"`
			Index  int    `json:"index"`
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output, &evaluation); err != nil {
		writeJSON(w, 500, apiError{"invalid evaluator output: " + string(output)})
		return
	}
	s.mu.Lock()
	for _, result := range evaluation.Results {
		id := result.ID
		if id == "" && result.Index > 0 && result.Index <= len(s.project.Config.Checks) {
			id = s.project.Config.Checks[result.Index-1].ID
		}
		progress := s.project.Progress[id]
		progress.LastResult, progress.LastObserved = result.Status, time.Now()
		if result.Status == "passing" {
			progress.SeenPassing = true
		}
		if result.Status == "failing" {
			progress.SeenFailing = true
		}
		s.project.Progress[id] = progress
	}
	project := s.project
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"evaluation": evaluation, "progress": project.Progress})
}

func (s *studioServer) handleRunScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.mu.RLock()
	var selected *ProjectScript
	for i := range s.project.Scripts {
		if s.project.Scripts[i].Name == request.Name {
			copyScript := s.project.Scripts[i]
			selected = &copyScript
			break
		}
	}
	s.mu.RUnlock()
	if selected == nil {
		writeJSON(w, 404, apiError{"script not found"})
		return
	}
	if selected.Platform != runtime.GOOS {
		writeJSON(w, 400, apiError{"script is for " + selected.Platform})
		return
	}
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", selected.Content)
	} else {
		command = exec.Command("/bin/bash", "-c", selected.Content)
	}
	command.Dir = s.targetDir
	output, err := command.CombinedOutput()
	status := 200
	if err != nil {
		status = 500
	}
	writeJSON(w, status, map[string]any{"output": string(output), "error": errorString(err)})
}

func (s *studioServer) handleRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		ConfirmUntested bool `json:"confirmUntested"`
		ConfirmCleanup  bool `json:"confirmCleanup"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeJSON(w, 400, apiError{err.Error()})
		return
	}
	s.mu.RLock()
	project := s.project
	s.mu.RUnlock()
	untested := []string{}
	for _, check := range project.Config.Checks {
		progress := project.Progress[check.ID]
		if !progress.SeenPassing || !progress.SeenFailing {
			untested = append(untested, check.Message)
		}
	}
	if len(untested) > 0 && !request.ConfirmUntested {
		writeJSON(w, 409, map[string]any{"error": "untested vulnerabilities require confirmation", "untested": untested})
		return
	}
	binary := filepath.Join(s.targetDir, runtimeBinaryName("aeacus"))
	args := []string{"--dir", withTrailingSeparator(s.targetDir), "studio-release"}
	command := exec.Command(binary, args...)
	command.Dir = s.targetDir
	cleanupResponse := "n\n"
	if request.ConfirmCleanup {
		cleanupResponse = "y\n"
	}
	command.Stdin = strings.NewReader("y\n" + cleanupResponse)
	output, err := command.CombinedOutput()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error(), "output": string(output)})
		return
	}
	writeJSON(w, 200, map[string]any{"message": "Release complete. Development Studio will remove itself after it exits.", "output": string(output)})
	go scheduleSelfRemoval(s.targetDir)
}

func (s *studioServer) loadProject(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &s.project); err != nil {
		return err
	}
	normalizeProject(&s.project)
	synchronizeForensicChecks(&s.project)
	ensureCheckIDs(&s.project.Config)
	if s.project.Progress == nil {
		s.project.Progress = map[string]Progress{}
	}
	return nil
}
func (s *studioServer) saveProject(path string) error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.project, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0600)
}

func defaultTargetDir() string {
	if runtime.GOOS == "windows" {
		return `C:\aeacus`
	}
	return "/opt/aeacus"
}
func runtimeBinaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
func withTrailingSeparator(path string) string {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}
func validateTarget(path string) error {
	clean := filepath.Clean(path)
	if clean == "." || clean == string(os.PathSeparator) || len(clean) < 4 {
		return errors.New("unsafe Aeacus target directory")
	}
	return nil
}
func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 20<<20)).Decode(value)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
}
func download(w http.ResponseWriter, name, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	_, _ = w.Write(data)
}
func randomToken() string {
	data := make([]byte, 24)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func atomicWrite(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".aeacus-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err = temp.Write(data); err == nil {
		err = temp.Chmod(mode)
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tempName, path)
}
func copyPath(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		return atomicWrite(to, data, info.Mode())
	}
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(from, path)
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, _ := entry.Info()
		return atomicWrite(target, data, info.Mode())
	})
}
func writeAudio(path, encoded string) error {
	if comma := strings.Index(encoded, ","); comma >= 0 {
		encoded = encoded[comma+1:]
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	if len(data) > 5<<20 {
		return errors.New("audio file exceeds 5 MiB")
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return errors.New("custom scoring sounds must be WAV files")
	}
	return atomicWrite(path, data, 0644)
}

func scheduleSelfRemoval(targetDir string) {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	bundleDir := filepath.Dir(executable)
	removeBundle := false
	if marker, markerErr := os.ReadFile(filepath.Join(bundleDir, ".aeacus-development-bundle")); markerErr == nil && strings.TrimSpace(string(marker)) == "AEACUS_STUDIO_DEVELOPMENT_BUNDLE_V3" && validateTarget(bundleDir) == nil {
		removeBundle = true
	}
	if runtime.GOOS == "windows" {
		cleanup := fmt.Sprintf("Start-Sleep -Seconds 3; Remove-Item -Force -ErrorAction SilentlyContinue %s", psQuote(filepath.Join(targetDir, "aeacus.exe")))
		if removeBundle {
			cleanup += fmt.Sprintf("; Remove-Item -Recurse -Force %s", psQuote(bundleDir))
		} else {
			cleanup += fmt.Sprintf("; Remove-Item -Force %s", psQuote(executable))
		}
		_ = exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command", cleanup).Start()
	} else {
		cleanupTarget := executable
		flag := "-f"
		if removeBundle {
			cleanupTarget, flag = bundleDir, "-rf"
		}
		_ = exec.Command("/bin/sh", "-c", "sleep 3; rm "+flag+" -- "+shellQuote(cleanupTarget)).Start()
	}
	time.Sleep(time.Second)
	os.Exit(0)
}
