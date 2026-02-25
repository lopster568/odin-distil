package ingester

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Chunk struct {
	Text      string
	FilePath  string
	Package   string
	Symbol    string
	Repo      string // top-level directory name under root
	DirPrefix string // first 3 path segments relative to root
}

func Walk(root string) ([]Chunk, error) {
	var chunks []Chunk
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// compute repo name and dir prefix relative to root
		rel, _ := filepath.Rel(root, path)
		parts := strings.SplitN(filepath.ToSlash(rel), "/", 4)
		repo := ""
		if len(parts) > 0 {
			repo = parts[0]
		}
		dirSegs := parts
		if len(dirSegs) > 3 {
			dirSegs = dirSegs[:3]
		}
		dirPrefix := strings.Join(dirSegs[:max(0, len(dirSegs)-1)], "/")

		var newChunks []Chunk
		switch {
		case strings.HasSuffix(path, ".go"):
			c, err := parseFile(path)
			if err != nil {
				return nil
			}
			newChunks = c
		case strings.HasSuffix(path, ".md"):
			newChunks = chunkMarkdown(path)
		}
		for i := range newChunks {
			newChunks[i].Repo = repo
			newChunks[i].DirPrefix = dirPrefix
		}
		chunks = append(chunks, newChunks...)
		return nil
	})
	return chunks, err
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func chunkMarkdown(path string) []Chunk {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// split on headings
	lines := strings.Split(string(src), "\n")
	var chunks []Chunk
	var current strings.Builder
	var heading string
	flush := func() {
		text := strings.TrimSpace(current.String())
		if len(text) > 50 {
			chunks = append(chunks, Chunk{
				Text:     text,
				FilePath: path,
				Package:  "docs",
				Symbol:   heading,
			})
		}
		current.Reset()
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			flush()
			heading = strings.TrimLeft(line, "# ")
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	flush()
	return chunks
}

func parseFile(path string) ([]Chunk, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return chunkRaw(path, string(src)), nil
	}
	pkgName := f.Name.Name
	var chunks []Chunk
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset
			if end > len(src) {
				end = len(src)
			}
			text := string(src[start:end])
			if d.Doc != nil {
				cs := fset.Position(d.Doc.Pos()).Offset
				text = string(src[cs:end])
			}
			symbol := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				symbol = fmt.Sprintf("%s.%s", receiverType(d.Recv.List[0].Type), symbol)
			}
			chunks = append(chunks, Chunk{Text: text, FilePath: path, Package: pkgName, Symbol: symbol})
		case *ast.GenDecl:
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset
			if end > len(src) {
				end = len(src)
			}
			chunks = append(chunks, Chunk{Text: string(src[start:end]), FilePath: path, Package: pkgName})
		}
	}
	if len(chunks) == 0 {
		return chunkRaw(path, string(src)), nil
	}
	return chunks, nil
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "Unknown"
}

func chunkRaw(path, src string) []Chunk {
	lines := strings.Split(src, "\n")
	const chunkSize = 100
	var chunks []Chunk
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, Chunk{
			Text:     strings.Join(lines[i:end], "\n"),
			FilePath: path,
			Package:  "unknown",
		})
	}
	return chunks
}
