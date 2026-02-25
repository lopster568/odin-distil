package distill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Bucket defines a Kubernetes subsystem and the path-prefix matchers used to
// filter Qdrant results into that bucket.
type Bucket struct {
	Name         string
	ProbeQuery   string
	PathPrefixes []string
}

var k8sBuckets = []Bucket{
	{
		Name:         "apiserver",
		ProbeQuery:   "API server request handling admission webhook authentication authorization",
		PathPrefixes: []string{"pkg/apiserver", "staging/src/k8s.io/apiserver"},
	},
	{
		Name:         "controllers",
		ProbeQuery:   "controller reconciliation loop desired state observed state workqueue informer",
		PathPrefixes: []string{"pkg/controller"},
	},
	{
		Name:         "scheduler",
		ProbeQuery:   "scheduler pod binding node selection priority preemption filter score",
		PathPrefixes: []string{"pkg/scheduler"},
	},
	{
		Name:         "kubelet",
		ProbeQuery:   "kubelet pod lifecycle container runtime node agent sync",
		PathPrefixes: []string{"pkg/kubelet"},
	},
	{
		Name:         "storage",
		ProbeQuery:   "storage etcd persistent volume claim watch list",
		PathPrefixes: []string{"pkg/storage", "staging/src/k8s.io/apiextensions-apiserver"},
	},
	{
		Name:         "admission",
		ProbeQuery:   "admission webhook validation mutation policy enforcement plugin",
		PathPrefixes: []string{"plugin/pkg/admission"},
	},
}

// ─── Stage 1: Bucket Retrieval ────────────────────────────────────────────────

// retrieveBuckets fetches top chunks per bucket from the vector store and
// post-filters by path prefix. Returns map[bucketName][]chunkText.
func (d *Distiller) retrieveBuckets(ctx context.Context) (map[string][]string, error) {
	buckets := make(map[string][]string, len(k8sBuckets))

	for _, b := range k8sBuckets {
		fmt.Printf("  [bucket] retrieving: %s\n", b.Name)
		vec, err := d.llm.Embed(ctx, b.ProbeQuery)
		if err != nil {
			return nil, fmt.Errorf("embed probe %s: %w", b.Name, err)
		}

		var allChunks []string
		for _, prefix := range b.PathPrefixes {
			results, err := d.store.SearchWithFilter(ctx, vec, 200, prefix)
			if err != nil {
				return nil, fmt.Errorf("search %s/%s: %w", b.Name, prefix, err)
			}
			for _, r := range results {
				allChunks = append(allChunks, fmt.Sprintf("// %s\n%s", r.FilePath, r.Text))
			}
		}

		fmt.Printf("  [bucket] %s → %d chunks\n", b.Name, len(allChunks))
		buckets[b.Name] = allChunks
	}
	return buckets, nil
}

// ─── Stage 2: Responsibility Extraction ──────────────────────────────────────

type Responsibility struct {
	Component           string   `json:"component"`
	CoreResponsibility  string   `json:"core_responsibility"`
	StateManaged        []string `json:"state_managed"`
	EventSources        []string `json:"event_sources"`
	ReconciliationLogic string   `json:"reconciliation_logic"`
	FailureHandling     string   `json:"failure_handling"`
}

func (d *Distiller) stageResponsibilities(ctx context.Context, buckets map[string][]string) error {
	outPath := filepath.Join(d.artifactsDir, "responsibilities.json")

	// Load existing partial results for resume support
	existing := map[string]Responsibility{}
	if data, err := os.ReadFile(outPath); err == nil {
		var list []Responsibility
		if json.Unmarshal(data, &list) == nil {
			for _, r := range list {
				existing[r.Component] = r
			}
		}
	}

	for _, b := range k8sBuckets {
		if _, done := existing[b.Name]; done {
			fmt.Printf("  [stage2] skip %s (already extracted)\n", b.Name)
			continue
		}

		chunks := buckets[b.Name]
		if len(chunks) == 0 {
			fmt.Printf("  [stage2] skip %s (no chunks)\n", b.Name)
			continue
		}

		// Batch chunks to ~4000 runes of context
		context := buildContext(chunks, 4000)
		prompt := responsibilityPrompt(b.Name, context)

		fmt.Printf("  [stage2] extracting responsibilities: %s\n", b.Name)
		t := time.Now()
		raw, err := d.llm.Generate(ctx, d.model, prompt)
		if err != nil {
			return fmt.Errorf("stage2 generate %s: %w", b.Name, err)
		}
		fmt.Printf("  [stage2] %s done in %s\n", b.Name, time.Since(t).Round(time.Second))

		raw = extractJSON(raw)
		var resp Responsibility
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			return fmt.Errorf("stage2 invalid JSON for %s: %w\nRaw: %s", b.Name, err, raw)
		}
		resp.Component = b.Name // enforce name
		existing[b.Name] = resp

		// Write intermediate result after each bucket
		if err := writeJSON(outPath, responsibilityMapToSlice(existing)); err != nil {
			return fmt.Errorf("stage2 write: %w", err)
		}
	}
	return nil
}

// ─── Stage 3: Pattern Mining ──────────────────────────────────────────────────

type Pattern struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
}

func (d *Distiller) stagePatterns(ctx context.Context) error {
	respPath := filepath.Join(d.artifactsDir, "responsibilities.json")
	outPath := filepath.Join(d.artifactsDir, "patterns.json")

	respData, err := os.ReadFile(respPath)
	if err != nil {
		return fmt.Errorf("stage3: cannot read responsibilities: %w", err)
	}

	prompt := patternPrompt(string(respData))
	fmt.Println("  [stage3] mining architectural patterns...")
	t := time.Now()
	raw, err := d.llm.Generate(ctx, d.model, prompt)
	if err != nil {
		return fmt.Errorf("stage3 generate: %w", err)
	}
	fmt.Printf("  [stage3] done in %s\n", time.Since(t).Round(time.Second))

	raw = extractJSON(raw)
	var patterns []Pattern
	if err := json.Unmarshal([]byte(raw), &patterns); err != nil {
		return fmt.Errorf("stage3 invalid JSON: %w\nRaw: %s", err, raw)
	}
	return writeJSON(outPath, patterns)
}

// ─── Stage 4: Reconciliation Loop Abstraction ─────────────────────────────────

func (d *Distiller) stageControlLoops(ctx context.Context) error {
	respPath := filepath.Join(d.artifactsDir, "responsibilities.json")
	outPath := filepath.Join(d.artifactsDir, "control_loops.md")

	respData, err := os.ReadFile(respPath)
	if err != nil {
		return fmt.Errorf("stage4: cannot read responsibilities: %w", err)
	}

	// Extract just the controllers component
	var list []Responsibility
	if err := json.Unmarshal(respData, &list); err != nil {
		return fmt.Errorf("stage4: parse responsibilities: %w", err)
	}
	ctrlJSON := "{}"
	for _, r := range list {
		if r.Component == "controllers" {
			b, _ := json.MarshalIndent(r, "", "  ")
			ctrlJSON = string(b)
			break
		}
	}

	prompt := controlLoopPrompt(ctrlJSON)
	fmt.Println("  [stage4] abstracting reconciliation loop...")
	t := time.Now()
	raw, err := d.llm.Generate(ctx, d.model, prompt)
	if err != nil {
		return fmt.Errorf("stage4 generate: %w", err)
	}
	fmt.Printf("  [stage4] done in %s\n", time.Since(t).Round(time.Second))

	if err := os.MkdirAll(d.artifactsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(raw), 0o644)
}

// ─── Stage 5: Friction & Opportunity Mining ───────────────────────────────────

type FrictionItem struct {
	Area         string `json:"area"`
	FrictionType string `json:"friction_type"`
	Rationale    string `json:"rationale"`
	Opportunity  string `json:"opportunity"`
}

func (d *Distiller) stageFriction(ctx context.Context) error {
	respData, err := os.ReadFile(filepath.Join(d.artifactsDir, "responsibilities.json"))
	if err != nil {
		return fmt.Errorf("stage5: cannot read responsibilities: %w", err)
	}
	patternsData, err := os.ReadFile(filepath.Join(d.artifactsDir, "patterns.json"))
	if err != nil {
		return fmt.Errorf("stage5: cannot read patterns: %w", err)
	}

	prompt := frictionPrompt(string(respData), string(patternsData))
	fmt.Println("  [stage5] mining friction and opportunities...")
	t := time.Now()
	raw, err := d.llm.Generate(ctx, d.model, prompt)
	if err != nil {
		return fmt.Errorf("stage5 generate: %w", err)
	}
	fmt.Printf("  [stage5] done in %s\n", time.Since(t).Round(time.Second))

	raw = extractJSON(raw)
	var items []FrictionItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return fmt.Errorf("stage5 invalid JSON: %w\nRaw: %s", err, raw)
	}
	return writeJSON(filepath.Join(d.artifactsDir, "friction.json"), items)
}

// ─── Stage 6: Study Topic Generation ─────────────────────────────────────────

func (d *Distiller) stageStudyTopics(ctx context.Context) error {
	respData, err := os.ReadFile(filepath.Join(d.artifactsDir, "responsibilities.json"))
	if err != nil {
		return fmt.Errorf("stage6: cannot read responsibilities: %w", err)
	}
	patternsData, err := os.ReadFile(filepath.Join(d.artifactsDir, "patterns.json"))
	if err != nil {
		return fmt.Errorf("stage6: cannot read patterns: %w", err)
	}
	frictionData, err := os.ReadFile(filepath.Join(d.artifactsDir, "friction.json"))
	if err != nil {
		return fmt.Errorf("stage6: cannot read friction: %w", err)
	}
	controlLoopsData, err := os.ReadFile(filepath.Join(d.artifactsDir, "control_loops.md"))
	if err != nil {
		return fmt.Errorf("stage6: cannot read control_loops: %w", err)
	}

	prompt := studyTopicsPrompt(
		string(respData), string(patternsData),
		string(frictionData), string(controlLoopsData),
	)
	fmt.Println("  [stage6] generating study topics...")
	t := time.Now()
	raw, err := d.llm.Generate(ctx, d.model, prompt)
	if err != nil {
		return fmt.Errorf("stage6 generate: %w", err)
	}
	fmt.Printf("  [stage6] done in %s\n", time.Since(t).Round(time.Second))

	if err := os.MkdirAll(d.artifactsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.artifactsDir, "study_topics.md"), []byte(raw), 0o644)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildContext concatenates chunks up to maxRunes.
func buildContext(chunks []string, maxRunes int) string {
	var sb strings.Builder
	for _, c := range chunks {
		if sb.Len()+len(c) > maxRunes {
			break
		}
		sb.WriteString(c)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// extractJSON attempts to pull the first JSON object or array from raw LLM output.
// It handles common LLM failure modes:
//   - markdown code fences wrapping the JSON
//   - array items returned without the opening '[' (starts with '{', ends with ']')
//   - leading prose before the first '[' or '{'
func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip markdown fences if present
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		raw = raw[idx+7:]
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		raw = raw[idx+3:]
	}
	if idx := strings.LastIndex(raw, "```"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)

	// Find the start of JSON — prefer '[' (array) before '{' (object) so that
	// array responses aren't accidentally trimmed to a single object.
	for _, start := range []string{"[", "{"} {
		if idx := strings.Index(raw, start); idx >= 0 {
			raw = raw[idx:]
			break
		}
	}
	raw = strings.TrimSpace(raw)

	// Heal: LLM sometimes emits array items without the opening '['.
	// Symptoms: starts with '{', ends with ']'.
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "]") {
		raw = "[" + raw
	}
	// Heal: LLM sometimes emits a bare array without the closing ']'.
	// Symptoms: starts with '[', doesn't end with ']' or '}'.
	if strings.HasPrefix(raw, "[") && !strings.HasSuffix(raw, "]") {
		raw = raw + "]"
	}

	return raw
}

// writeJSON marshals v and writes to path with MkdirAll.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func responsibilityMapToSlice(m map[string]Responsibility) []Responsibility {
	out := make([]Responsibility, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
