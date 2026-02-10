package context

import (
	"os"
	"path/filepath"
	"strings"
)

// Detector identifies project type from files
type Detector struct {
	Name     string
	Files    []string
	Patterns []string
}

var detectors = []Detector{
	{Name: "go", Files: []string{"go.mod"}, Patterns: []string{"*.go"}},
	{Name: "python", Files: []string{"requirements.txt", "pyproject.toml", "setup.py", "Pipfile"}, Patterns: []string{"*.py"}},
	{Name: "node", Files: []string{"package.json"}, Patterns: []string{"*.js", "*.ts", "*.jsx", "*.tsx"}},
	{Name: "rust", Files: []string{"Cargo.toml"}, Patterns: []string{"*.rs"}},
	{Name: "ruby", Files: []string{"Gemfile", "*.gemspec"}, Patterns: []string{"*.rb"}},
	{Name: "java", Files: []string{"pom.xml", "build.gradle"}, Patterns: []string{"*.java"}},
	{Name: "docker", Files: []string{"Dockerfile", "docker-compose.yml"}, Patterns: []string{"Dockerfile*"}},
}

// DetectType identifies project type
func DetectType(root string) string {
	for _, d := range detectors {
		for _, f := range d.Files {
			if strings.Contains(f, "*") {
				if matches, _ := filepath.Glob(filepath.Join(root, f)); len(matches) > 0 {
					return d.Name
				}
			} else if _, err := os.Stat(filepath.Join(root, f)); err == nil {
				return d.Name
			}
		}
	}
	return "unknown"
}

// AnalyzeStructure detects tech stack from project files
func AnalyzeStructure(root string) TechStack {
	var ts TechStack

	// Detect languages by file extensions
	extCount := make(map[string]int)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(info.Name())
		if ext != "" {
			extCount[ext]++
		}
		return nil
	})

	langMap := map[string]string{
		".go": "Go", ".py": "Python", ".js": "JavaScript",
		".ts": "TypeScript", ".rs": "Rust", ".rb": "Ruby",
		".java": "Java", ".c": "C", ".cpp": "C++",
		".h": "C/C++", ".hpp": "C++", ".cs": "C#",
		".php": "PHP", ".swift": "Swift", ".kt": "Kotlin",
	}

	for ext, lang := range langMap {
		if extCount[ext] > 0 {
			ts.Languages = append(ts.Languages, lang)
		}
	}

	// Detect frameworks and build tools
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		ts.Runtimes = append(ts.Runtimes, "Go")
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		ts.Runtimes = append(ts.Runtimes, "Node.js")
		data, _ := os.ReadFile(filepath.Join(root, "package.json"))
		content := string(data)
		if strings.Contains(content, "react") {
			ts.Frameworks = append(ts.Frameworks, "React")
		}
		if strings.Contains(content, "vue") {
			ts.Frameworks = append(ts.Frameworks, "Vue")
		}
		if strings.Contains(content, "next") {
			ts.Frameworks = append(ts.Frameworks, "Next.js")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err == nil {
		ts.Runtimes = append(ts.Runtimes, "Rust")
	}
	if _, err := os.Stat(filepath.Join(root, "requirements.txt")); err == nil {
		ts.Runtimes = append(ts.Runtimes, "Python")
	}

	return ts
}

// DetectCommands extracts common commands from package files
func DetectCommands(root, pType string) CommonCommands {
	var cmds CommonCommands

	switch pType {
	case "go":
		cmds.Build = []string{"go build", "go build ./..."}
		cmds.Test = []string{"go test ./...", "go test -v ./..."}
		cmds.Dev = []string{"go run ."}
	case "node":
		if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
			content := string(data)
			if strings.Contains(content, `"build"`) {
				cmds.Build = append(cmds.Build, "npm run build", "yarn build")
			}
			if strings.Contains(content, `"test"`) {
				cmds.Test = append(cmds.Test, "npm test", "yarn test")
			}
			if strings.Contains(content, `"dev"`) || strings.Contains(content, `"start"`) {
				cmds.Dev = append(cmds.Dev, "npm run dev", "npm start")
			}
			if strings.Contains(content, `"lint"`) {
				cmds.Lint = append(cmds.Lint, "npm run lint")
			}
		}
	case "rust":
		cmds.Build = []string{"cargo build", "cargo build --release"}
		cmds.Test = []string{"cargo test"}
		cmds.Dev = []string{"cargo run"}
	case "python":
		cmds.Build = []string{"pip install -r requirements.txt"}
		cmds.Test = []string{"pytest", "python -m pytest"}
		if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
			cmds.Build = append([]string{"pip install -e ."}, cmds.Build...)
		}
	}

	// Check for Makefile
	if _, err := os.Stat(filepath.Join(root, "Makefile")); err == nil {
		cmds.Custom = append(cmds.Custom, "make", "make build", "make test")
	}

	return cmds
}
