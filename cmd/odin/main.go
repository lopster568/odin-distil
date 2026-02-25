package main

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strings"

	"odin/internal/distill"
	"odin/internal/embedder"
	"odin/internal/ingester"
	"odin/internal/llm"
	"odin/internal/query"
	"odin/internal/store"
)

const repoRoot = "/root/repos"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: odin <ingest|distill|ask> [args]")
		fmt.Println("  odin ingest <path>      - index a Go source tree")
		fmt.Println("  odin distill [k8s]      - run architecture distillation pipeline")
		fmt.Println("  odin ask                - interactive agent session")
		os.Exit(1)
	}

	ctx := context.Background()

	llmClient, err := llm.New()
	must(err, "llm init")

	st, err := store.New()
	must(err, "store init")

	must(st.EnsureCollection(ctx), "ensure collection")

	switch os.Args[1] {
	case "ingest":
		if len(os.Args) < 3 {
			log.Fatal("usage: odin ingest <path>")
		}
		runIngest(ctx, os.Args[2], llmClient, st)
	case "distill":
		target := "k8s"
		if len(os.Args) >= 3 {
			target = os.Args[2]
		}
		artifactsDir := fmt.Sprintf("artifacts/%s", target)
		d := distill.New(llmClient, st, artifactsDir)
		must(d.Run(ctx), "distill")
	case "ask":
		runAgent(ctx, llmClient, st)
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func runIngest(ctx context.Context, root string, llmClient *llm.Client, st *store.Store) {
	fmt.Printf("Ingesting %s ...\n", root)
	chunks, err := ingester.Walk(root)
	must(err, "walk")
	fmt.Printf("Found %d chunks, embedding...\n", len(chunks))

	emb := embedder.New(llmClient)
	batch := 50
	total := 0

	for i := 0; i < len(chunks); i += batch {
		end := i + batch
		if end > len(chunks) {
			end = len(chunks)
		}
		sc, err := emb.EmbedChunks(ctx, chunks[i:end])
		if err != nil {
			fmt.Printf("  warn: batch %d failed: %v\n", i/batch, err)
			continue
		}
		storeChunks := make([]store.Chunk, len(sc))
		for j, c := range sc {
			storeChunks[j] = store.Chunk{
				ID:        hash(c.FilePath + c.Symbol + fmt.Sprint(i+j)),
				Text:      c.Text,
				FilePath:  c.FilePath,
				Package:   c.Package,
				Symbol:    c.Symbol,
				Repo:      c.Repo,
				DirPrefix: c.DirPrefix,
				Vector:    c.Vector,
			}
		}
		if err := st.Upsert(ctx, storeChunks); err != nil {
			fmt.Printf("  warn: upsert batch %d failed: %v\n", i/batch, err)
			continue
		}
		total += len(sc)
		fmt.Printf("  indexed %d / %d\n", total, len(chunks))
	}
	fmt.Printf("Done. Indexed %d chunks.\n", total)
}

func runAgent(ctx context.Context, llmClient *llm.Client, st *store.Store) {
	agent := query.NewAgent(llmClient, st, repoRoot)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║     ODIN — Kubernetes Intelligence    ║")
	fmt.Println("╠═══════════════════════════════════════╣")
	fmt.Println("║  Tools: grep_symbol, get_file,        ║")
	fmt.Println("║         list_package                  ║")
	fmt.Println("║  Commands: /clear  /quit              ║")
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Println()

	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch input {
		case "/clear":
			agent.ClearHistory()
			fmt.Println("  conversation cleared")
			continue
		case "/quit", "/exit":
			fmt.Println("bye")
			return
		}

		fmt.Println()
		answer, err := agent.Ask(ctx, input)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		fmt.Println(answer)
		fmt.Println()
	}
}

func hash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func must(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}
