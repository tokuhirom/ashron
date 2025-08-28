package tools

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GenerateAgentsMD generates an AGENTS.md file for the project
func GenerateAgentsMD(rootPath string) (string, error) {
	slog.Info("Generating AGENTS.md file",
		slog.String("rootPath", rootPath))

	if rootPath == "" {
		rootPath = "."
	}

	// Get project structure
	structure, err := getProjectStructure(rootPath)
	if err != nil {
		return "", fmt.Errorf("failed to get project structure: %w", err)
	}

	// Check if AGENTS.md already exists
	agentsPath := filepath.Join(rootPath, "AGENTS.md")
	var existingContent string
	if data, err := os.ReadFile(agentsPath); err == nil {
		existingContent = string(data)
	}

	// Generate new AGENTS.md content
	content := generateAgentsMDContent(rootPath, structure, existingContent)

	// Write the file
	if err := os.WriteFile(agentsPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write AGENTS.md: %w", err)
	}

	return content, nil
}

type fileInfo struct {
	Path     string
	IsDir    bool
	Size     int64
	Language string
}

func getProjectStructure(rootPath string) ([]fileInfo, error) {
	var files []fileInfo

	// Common ignore patterns
	ignorePatterns := []string{
		".git", "node_modules", "vendor", ".vscode", ".idea",
		"dist", "build", "target", "__pycache__", ".pytest_cache",
		"*.pyc", "*.pyo", "*.log", "*.tmp", ".DS_Store",
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get a relative path
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}

		// Check if we should ignore
		for _, pattern := range ignorePatterns {
			if matched, _ := filepath.Match(pattern, d.Name()); matched {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.Contains(relPath, "/"+pattern+"/") {
				return nil
			}
		}

		// Skip hidden files/dirs (except .github, .gitignore, etc.)
		if strings.HasPrefix(d.Name(), ".") &&
			!strings.HasPrefix(d.Name(), ".git") &&
			d.Name() != ".gitignore" &&
			d.Name() != ".env.example" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		fi := fileInfo{
			Path:  relPath,
			IsDir: d.IsDir(),
			Size:  info.Size(),
		}

		// Detect language by extension
		if !d.IsDir() {
			fi.Language = detectLanguage(path)
		}

		files = append(files, fi)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files by path
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".java":
		return "Java"
	case ".c":
		return "C"
	case ".cpp", ".cc", ".cxx":
		return "C++"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".swift":
		return "Swift"
	case ".kt", ".kts":
		return "Kotlin"
	case ".cs":
		return "C#"
	case ".sh", ".bash":
		return "Shell"
	case ".yaml", ".yml":
		return "YAML"
	case ".json":
		return "JSON"
	case ".xml":
		return "XML"
	case ".md":
		return "Markdown"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS"
	case ".scss", ".sass":
		return "SCSS"
	case ".sql":
		return "SQL"
	case ".dockerfile":
		return "Docker"
	default:
		if strings.HasSuffix(path, "Dockerfile") {
			return "Docker"
		}
		if strings.HasSuffix(path, "Makefile") {
			return "Make"
		}
		return ""
	}
}

func generateAgentsMDContent(rootPath string, files []fileInfo, existingContent string) string {
	var sb strings.Builder

	// Detect a project type and main language
	projectType, mainLang := detectProjectType(files)

	// Parse existing content for custom sections
	customSections := parseCustomSections(existingContent)

	// Header
	sb.WriteString("# AGENTS.md\n\n")
	sb.WriteString("## Project Overview\n\n")

	// Project name (from directory name)
	projectName := filepath.Base(rootPath)
	if absPath, err := filepath.Abs(rootPath); err == nil {
		projectName = filepath.Base(absPath)
	}
	sb.WriteString(fmt.Sprintf("**Project:** %s\n", projectName))
	sb.WriteString(fmt.Sprintf("**Type:** %s\n", projectType))
	sb.WriteString(fmt.Sprintf("**Primary Language:** %s\n\n", mainLang))

	// Custom description if exists
	if desc, exists := customSections["description"]; exists {
		sb.WriteString("### Description\n\n")
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}

	// Project Structure
	sb.WriteString("## Project Structure\n\n")
	sb.WriteString("```\n")
	sb.WriteString(generateTreeStructure(files))
	sb.WriteString("```\n\n")

	// Key Files and Directories
	sb.WriteString("## Key Components\n\n")
	keyComponents := identifyKeyComponents(files, projectType)
	for category, items := range keyComponents {
		if len(items) > 0 {
			sb.WriteString(fmt.Sprintf("### %s\n\n", category))
			for _, item := range items {
				sb.WriteString(fmt.Sprintf("- `%s`", item.Path))
				if item.Description != "" {
					sb.WriteString(fmt.Sprintf(": %s", item.Description))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// Technologies Used
	sb.WriteString("## Technologies\n\n")
	techs := detectTechnologies(files)
	for _, tech := range techs {
		sb.WriteString(fmt.Sprintf("- %s\n", tech))
	}
	sb.WriteString("\n")

	// Development Guidelines
	sb.WriteString("## Development Guidelines\n\n")

	if guidelines, exists := customSections["guidelines"]; exists {
		sb.WriteString(guidelines)
	} else {
		sb.WriteString("### Code Style\n\n")
		sb.WriteString(fmt.Sprintf("- Follow standard %s conventions\n", mainLang))
		sb.WriteString("- Maintain consistent formatting\n")
		sb.WriteString("- Write clear, self-documenting code\n\n")

		sb.WriteString("### Testing\n\n")
		sb.WriteString("- Write tests for new features\n")
		sb.WriteString("- Ensure all tests pass before committing\n\n")

		sb.WriteString("### Documentation\n\n")
		sb.WriteString("- Update documentation for API changes\n")
		sb.WriteString("- Include comments for complex logic\n")
	}
	sb.WriteString("\n")

	// Agent Instructions
	sb.WriteString("## AI Agent Instructions\n\n")

	if instructions, exists := customSections["instructions"]; exists {
		sb.WriteString(instructions)
	} else {
		sb.WriteString("When working on this project:\n\n")
		sb.WriteString("1. **Read First**: Review relevant code before making changes\n")
		sb.WriteString("2. **Test Changes**: Run tests after modifications\n")
		sb.WriteString("3. **Follow Patterns**: Maintain consistency with existing code style\n")
		sb.WriteString("4. **Document**: Update documentation and comments as needed\n")
		sb.WriteString("5. **Validate**: Ensure changes don't break existing functionality\n")
	}
	sb.WriteString("\n")

	// Auto-generated notice
	sb.WriteString("---\n")
	sb.WriteString("*This file was auto-generated by Ashron. Custom sections are preserved during updates.*\n")

	return sb.String()
}

func detectProjectType(files []fileInfo) (string, string) {
	// Count files by extension
	langCount := make(map[string]int)
	hasPackageJSON := false
	hasGoMod := false
	hasPyproject := false
	hasCargoToml := false
	hasPomXML := false
	hasGemfile := false

	for _, f := range files {
		if f.Language != "" && !f.IsDir {
			langCount[f.Language]++
		}

		switch f.Path {
		case "package.json":
			hasPackageJSON = true
		case "go.mod":
			hasGoMod = true
		case "pyproject.toml", "setup.py":
			hasPyproject = true
		case "Cargo.toml":
			hasCargoToml = true
		case "pom.xml":
			hasPomXML = true
		case "Gemfile":
			hasGemfile = true
		}
	}

	// Determine main language
	var mainLang string
	maxCount := 0
	for lang, count := range langCount {
		if count > maxCount {
			maxCount = count
			mainLang = lang
		}
	}

	// Determine project type
	var projectType string
	if hasGoMod {
		projectType = "Go Application"
		mainLang = "Go"
	} else if hasPackageJSON {
		projectType = "Node.js/JavaScript Project"
		if langCount["TypeScript"] > langCount["JavaScript"] {
			mainLang = "TypeScript"
		} else {
			mainLang = "JavaScript"
		}
	} else if hasPyproject {
		projectType = "Python Project"
		mainLang = "Python"
	} else if hasCargoToml {
		projectType = "Rust Project"
		mainLang = "Rust"
	} else if hasPomXML {
		projectType = "Java/Maven Project"
		mainLang = "Java"
	} else if hasGemfile {
		projectType = "Ruby Project"
		mainLang = "Ruby"
	} else {
		projectType = "General Project"
	}

	return projectType, mainLang
}

type keyComponent struct {
	Path        string
	Description string
}

func identifyKeyComponents(files []fileInfo, projectType string) map[string][]keyComponent {
	components := make(map[string][]keyComponent)

	for _, f := range files {
		if f.IsDir {
			// Identify key directories
			switch {
			case strings.HasSuffix(f.Path, "/cmd"):
				components["Entry Points"] = append(components["Entry Points"],
					keyComponent{f.Path, "Command-line applications"})
			case strings.HasSuffix(f.Path, "/internal"):
				components["Core Logic"] = append(components["Core Logic"],
					keyComponent{f.Path, "Internal packages"})
			case strings.HasSuffix(f.Path, "/pkg"):
				components["Public Libraries"] = append(components["Public Libraries"],
					keyComponent{f.Path, "Public packages"})
			case strings.HasSuffix(f.Path, "/src"):
				components["Source Code"] = append(components["Source Code"],
					keyComponent{f.Path, "Main source directory"})
			case strings.HasSuffix(f.Path, "/test"), strings.HasSuffix(f.Path, "/tests"):
				components["Testing"] = append(components["Testing"],
					keyComponent{f.Path, "Test files"})
			case strings.HasSuffix(f.Path, "/docs"):
				components["Documentation"] = append(components["Documentation"],
					keyComponent{f.Path, "Project documentation"})
			}
		} else {
			// Identify key files
			switch filepath.Base(f.Path) {
			case "main.go", "main.py", "index.js", "index.ts", "main.rs":
				components["Entry Points"] = append(components["Entry Points"],
					keyComponent{f.Path, "Main entry point"})
			case "go.mod", "package.json", "Cargo.toml", "pyproject.toml":
				components["Configuration"] = append(components["Configuration"],
					keyComponent{f.Path, "Project configuration"})
			case "Makefile":
				components["Build"] = append(components["Build"],
					keyComponent{f.Path, "Build configuration"})
			case "Dockerfile":
				components["Deployment"] = append(components["Deployment"],
					keyComponent{f.Path, "Container configuration"})
			case "README.md":
				components["Documentation"] = append(components["Documentation"],
					keyComponent{f.Path, "Project documentation"})
			case ".github/workflows/ci.yml", ".gitlab-ci.yml":
				components["CI/CD"] = append(components["CI/CD"],
					keyComponent{f.Path, "Continuous integration"})
			}
		}
	}

	return components
}

func detectTechnologies(files []fileInfo) []string {
	techs := make(map[string]bool)

	for _, f := range files {
		switch {
		case strings.HasSuffix(f.Path, "docker-compose.yml"):
			techs["Docker Compose"] = true
		case strings.HasSuffix(f.Path, "Dockerfile"):
			techs["Docker"] = true
		case strings.HasSuffix(f.Path, ".github/workflows"):
			techs["GitHub Actions"] = true
		case strings.HasSuffix(f.Path, "Makefile"):
			techs["Make"] = true
		case strings.HasSuffix(f.Path, "go.mod"):
			techs["Go Modules"] = true
		case strings.HasSuffix(f.Path, "package.json"):
			techs["npm/yarn"] = true
		case strings.HasSuffix(f.Path, ".proto"):
			techs["Protocol Buffers"] = true
		case strings.HasSuffix(f.Path, ".graphql"):
			techs["GraphQL"] = true
		}
	}

	var result []string
	for tech := range techs {
		result = append(result, tech)
	}
	sort.Strings(result)
	return result
}

func generateTreeStructure(files []fileInfo) string {
	var sb strings.Builder

	// Build tree structure
	type node struct {
		name     string
		children map[string]*node
		isFile   bool
	}

	root := &node{children: make(map[string]*node)}

	for _, f := range files {
		if f.Path == "." {
			continue
		}

		parts := strings.Split(f.Path, string(filepath.Separator))
		current := root

		for i, part := range parts {
			if current.children[part] == nil {
				current.children[part] = &node{
					name:     part,
					children: make(map[string]*node),
					isFile:   i == len(parts)-1 && !f.IsDir,
				}
			}
			current = current.children[part]
		}
	}

	// Print tree
	var printNode func(n *node, prefix string, isLast bool)
	printNode = func(n *node, prefix string, isLast bool) {
		if n.name != "" {
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			sb.WriteString(prefix + connector + n.name)
			if !n.isFile && len(n.children) > 0 {
				sb.WriteString("/")
			}
			sb.WriteString("\n")
		}

		// Skip deep nesting
		if strings.Count(prefix, "│") > 3 {
			if len(n.children) > 0 {
				sb.WriteString(prefix)
				if !isLast {
					sb.WriteString("│   ")
				} else {
					sb.WriteString("    ")
				}
				sb.WriteString("└── ...\n")
			}
			return
		}

		keys := make([]string, 0, len(n.children))
		for k := range n.children {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			child := n.children[k]
			newPrefix := prefix
			if n.name != "" {
				if isLast {
					newPrefix += "    "
				} else {
					newPrefix += "│   "
				}
			}
			printNode(child, newPrefix, i == len(keys)-1)
		}
	}

	printNode(root, "", false)

	return sb.String()
}

func parseCustomSections(content string) map[string]string {
	sections := make(map[string]string)

	if content == "" {
		return sections
	}

	// Look for custom sections marked with special comments
	// <!-- CUSTOM:description -->...<!-- /CUSTOM:description -->

	customPattern := `<!-- CUSTOM:(\w+) -->(.*?)<!-- /CUSTOM:\w+ -->`
	re := regexp.MustCompile(customPattern)

	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			sections[match[1]] = strings.TrimSpace(match[2])
		}
	}

	return sections
}
