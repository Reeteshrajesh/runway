package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Reeteshrajesh/runway/internal/color"
)

// runtimeHints detects the project runtime from files in cwd and returns
// suggested setup/build/start commands.
type runtimeHints struct {
	name  string
	setup []string
	build []string
	start []string
}

func detectRuntime(dir string) runtimeHints {
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	switch {
	case has("package.json"):
		return runtimeHints{
			name:  "Node.js",
			setup: []string{"npm install"},
			build: []string{"npm run build"},
			start: []string{"pm2 restart app || pm2 start dist/index.js --name app"},
		}
	case has("requirements.txt") || has("pyproject.toml"):
		return runtimeHints{
			name:  "Python",
			setup: []string{"pip install -r requirements.txt"},
			build: []string{},
			start: []string{"systemctl restart myapp"},
		}
	case has("go.mod"):
		return runtimeHints{
			name:  "Go",
			setup: []string{"go mod download"},
			build: []string{"go build -o bin/app ./cmd/app"},
			start: []string{"systemctl restart myapp"},
		}
	case has("Gemfile"):
		return runtimeHints{
			name:  "Ruby",
			setup: []string{"bundle install"},
			build: []string{},
			start: []string{"systemctl restart myapp"},
		}
	default:
		return runtimeHints{
			name:  "unknown",
			setup: []string{},
			build: []string{},
			start: []string{"systemctl restart myapp"},
		}
	}
}

func runInit(args []string) error {
	fs := newFlagSet("init")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway init")
		fmt.Fprintln(os.Stderr, "\nInteractively create manifest.yml and the deployment directory structure.")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	ask := func(prompt, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("  %s [%s]: ", prompt, defaultVal)
		} else {
			fmt.Printf("  %s: ", prompt)
		}
		scanner.Scan()
		val := strings.TrimSpace(scanner.Text())
		if val == "" {
			return defaultVal
		}
		return val
	}

	fmt.Printf("\n %s\n\n", color.Bold("runway init"))

	// Detect runtime from the current directory.
	cwd, _ := os.Getwd()
	hints := detectRuntime(cwd)
	if hints.name != "unknown" {
		color.Infof(os.Stdout, "Detected: %s", hints.name)
		fmt.Println()
	}

	appName := ask("App name", filepath.Base(cwd))
	repoURL := ask("Git repo URL", os.Getenv("GITOPS_REPO"))
	deployDir := ask("Deploy directory", "/opt/runway")
	port := ask("Webhook port", "9000")

	fmt.Println()

	// Prompt for commands with detected defaults.
	setupStr := ask("Setup commands", strings.Join(hints.setup, ", "))
	buildStr := ask("Build commands", strings.Join(hints.build, ", "))
	startStr := ask("Start commands", strings.Join(hints.start, ", "))

	// Build manifest content.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("app: %s\n", appName))
	sb.WriteString("timeout: 600\n")
	sb.WriteString(fmt.Sprintf("env_file: %s/.env\n", deployDir))

	writeCommands := func(key, input string) {
		cmds := splitCommands(input)
		if len(cmds) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("%s:\n", key))
		for _, c := range cmds {
			sb.WriteString(fmt.Sprintf("  - %s\n", c))
		}
	}

	writeCommands("setup", setupStr)
	writeCommands("build", buildStr)
	writeCommands("start", startStr)

	// Write manifest.yml.
	manifestPath := filepath.Join(deployDir, "manifest.yml")
	releasesDir := filepath.Join(deployDir, "releases")
	envFile := filepath.Join(deployDir, ".env")

	fmt.Println()

	// Create directory structure.
	if err := os.MkdirAll(releasesDir, 0755); err != nil {
		color.Errorf(os.Stdout, "could not create %s: %v", releasesDir, err)
		return err
	}
	color.Successf(os.Stdout, "%s created", releasesDir)

	if err := os.WriteFile(manifestPath, []byte(sb.String()), 0644); err != nil {
		color.Errorf(os.Stdout, "could not write %s: %v", manifestPath, err)
		return err
	}
	color.Successf(os.Stdout, "manifest.yml created (%s)", manifestPath)

	// Create empty .env if it doesn't exist.
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		if err := os.WriteFile(envFile, []byte("# runway environment variables\n"), 0600); err != nil {
			color.Warnf(os.Stdout, "could not create %s: %v", envFile, err)
		} else {
			color.Successf(os.Stdout, ".env created (%s)", envFile)
		}
	} else {
		color.Successf(os.Stdout, ".env already exists (%s)", envFile)
	}

	_ = repoURL // used in next steps output

	fmt.Printf("\n %s\n\n", color.Bold("Next steps:"))
	fmt.Printf("   1. Add your secrets to %s\n", envFile)
	fmt.Printf("   2. Set environment variable: export GITOPS_REPO=%s\n", repoURL)
	fmt.Printf("   3. Add webhook in GitHub/GitLab:  http://your-server:%s/webhook\n", port)
	fmt.Printf("   4. Run: runway listen --port %s --secret <your-secret>\n\n", port)

	return nil
}

// splitCommands splits a comma-separated or single command string into a slice.
func splitCommands(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
