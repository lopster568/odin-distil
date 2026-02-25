package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Result struct {
	Tool   string
	Input  string
	Output string
	Error  string
}

func GrepSymbol(symbol string, repoRoot string) Result {
	cmd := exec.Command("grep", "-r", "--include=*.go",
		"-n", "--max-count=5", "-l", symbol, repoRoot)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return Result{Tool: "grep_symbol", Input: symbol, Output: "no matches found"}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sb strings.Builder
	for i, file := range lines {
		if i >= 5 || file == "" {
			break
		}
		ctx := exec.Command("grep", "-n", "--max-count=3", symbol, file)
		ctxOut, _ := ctx.Output()
		sb.WriteString(fmt.Sprintf("── %s\n%s\n", file, string(ctxOut)))
	}
	return Result{Tool: "grep_symbol", Input: symbol, Output: sb.String()}
}

func GetFile(path string) Result {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Result{Tool: "get_file", Input: path, Error: "invalid path"}
	}
	allowed := false
	for _, prefix := range []string{"/root/repos", "/root/odin"} {
		if strings.HasPrefix(abs, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return Result{Tool: "get_file", Input: path, Error: "path not allowed"}
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return Result{Tool: "get_file", Input: path, Error: err.Error()}
	}
	content := string(src)
	if len(content) > 8000 {
		content = content[:8000] + "\n... (truncated)"
	}
	return Result{Tool: "get_file", Input: path, Output: content}
}

func ListPackage(pkgPath string) Result {
	cmd := exec.Command("grep", "-r", "--include=*.go",
		"-h", "-E", "^func |^type |^var |^const ", pkgPath)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return Result{Tool: "list_package", Input: pkgPath, Error: "no go files found"}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	seen := map[string]bool{}
	var unique []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !seen[l] {
			seen[l] = true
			unique = append(unique, l)
		}
	}
	if len(unique) > 80 {
		unique = unique[:80]
	}
	return Result{Tool: "list_package", Input: pkgPath, Output: strings.Join(unique, "\n")}
}
