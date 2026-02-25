package distill

import (
	"context"
	"fmt"
	"time"

	"odin/internal/llm"
	"odin/internal/store"
)

const defaultModel = "qwen2.5:72b"

// Distiller runs the 6-stage Kubernetes architecture distillation pipeline.
type Distiller struct {
	llm          *llm.Client
	store        *store.Store
	artifactsDir string
	model        string
}

// New creates a Distiller. artifactsDir is where all output files are written
// (e.g. "artifacts/k8s").
func New(l *llm.Client, s *store.Store, artifactsDir string) *Distiller {
	return &Distiller{
		llm:          l,
		store:        s,
		artifactsDir: artifactsDir,
		model:        defaultModel,
	}
}

// Run executes the full pipeline, skipping stages already recorded in checkpoint.json.
func (d *Distiller) Run(ctx context.Context) error {
	cp, err := loadCheckpoint(d.artifactsDir)
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}

	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║   ODIN DISTILL — Kubernetes Pipeline  ║")
	fmt.Printf("║   Artifacts: %-25s║\n", d.artifactsDir)
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Println()

	// ── Stage 1: Bucket Retrieval (always runs — ephemeral) ──────────────────
	fmt.Println("▶ Stage 1/6 — Bucket Retrieval")
	t1 := time.Now()
	buckets, err := d.retrieveBuckets(ctx)
	if err != nil {
		return fmt.Errorf("stage1: %w", err)
	}
	fmt.Printf("  ✓ done in %s\n\n", time.Since(t1).Round(time.Second))

	// ── Stage 2: Responsibility Extraction ────────────────────────────────────
	if cp.done("responsibilities") {
		fmt.Println("▶ Stage 2/6 — Responsibility Extraction [SKIPPED — checkpoint]")
	} else {
		fmt.Println("▶ Stage 2/6 — Responsibility Extraction")
		t := time.Now()
		if err := d.stageResponsibilities(ctx, buckets); err != nil {
			return fmt.Errorf("stage2: %w", err)
		}
		if err := cp.mark("responsibilities", d.artifactsDir); err != nil {
			return err
		}
		fmt.Printf("  ✓ done in %s → responsibilities.json\n\n", time.Since(t).Round(time.Second))
	}

	// ── Stage 3: Pattern Mining ───────────────────────────────────────────────
	if cp.done("patterns") {
		fmt.Println("▶ Stage 3/6 — Pattern Mining [SKIPPED — checkpoint]")
	} else {
		fmt.Println("▶ Stage 3/6 — Pattern Mining")
		t := time.Now()
		if err := d.stagePatterns(ctx); err != nil {
			return fmt.Errorf("stage3: %w", err)
		}
		if err := cp.mark("patterns", d.artifactsDir); err != nil {
			return err
		}
		fmt.Printf("  ✓ done in %s → patterns.json\n\n", time.Since(t).Round(time.Second))
	}

	// ── Stage 4: Reconciliation Loop Abstraction ──────────────────────────────
	if cp.done("control_loops") {
		fmt.Println("▶ Stage 4/6 — Control Loop Abstraction [SKIPPED — checkpoint]")
	} else {
		fmt.Println("▶ Stage 4/6 — Control Loop Abstraction")
		t := time.Now()
		if err := d.stageControlLoops(ctx); err != nil {
			return fmt.Errorf("stage4: %w", err)
		}
		if err := cp.mark("control_loops", d.artifactsDir); err != nil {
			return err
		}
		fmt.Printf("  ✓ done in %s → control_loops.md\n\n", time.Since(t).Round(time.Second))
	}

	// ── Stage 5: Friction & Opportunity Mining ────────────────────────────────
	if cp.done("friction") {
		fmt.Println("▶ Stage 5/6 — Friction Mining [SKIPPED — checkpoint]")
	} else {
		fmt.Println("▶ Stage 5/6 — Friction Mining")
		t := time.Now()
		if err := d.stageFriction(ctx); err != nil {
			return fmt.Errorf("stage5: %w", err)
		}
		if err := cp.mark("friction", d.artifactsDir); err != nil {
			return err
		}
		fmt.Printf("  ✓ done in %s → friction.json\n\n", time.Since(t).Round(time.Second))
	}

	// ── Stage 6: Study Topic Generation ──────────────────────────────────────
	if cp.done("study_topics") {
		fmt.Println("▶ Stage 6/6 — Study Topics [SKIPPED — checkpoint]")
	} else {
		fmt.Println("▶ Stage 6/6 — Study Topic Generation")
		t := time.Now()
		if err := d.stageStudyTopics(ctx); err != nil {
			return fmt.Errorf("stage6: %w", err)
		}
		if err := cp.mark("study_topics", d.artifactsDir); err != nil {
			return err
		}
		fmt.Printf("  ✓ done in %s → study_topics.md\n\n", time.Since(t).Round(time.Second))
	}

	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║           DISTILLATION COMPLETE       ║")
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Printf("\nArtifacts written to: %s\n", d.artifactsDir)
	return nil
}
