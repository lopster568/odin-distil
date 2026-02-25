package distill

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const checkpointFile = "checkpoint.json"

// Checkpoint tracks which pipeline stages have completed successfully.
type Checkpoint struct {
	Completed map[string]bool `json:"completed"`
}

func loadCheckpoint(artifactsDir string) (*Checkpoint, error) {
	path := filepath.Join(artifactsDir, checkpointFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Checkpoint{Completed: map[string]bool{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	if cp.Completed == nil {
		cp.Completed = map[string]bool{}
	}
	return &cp, nil
}

func (cp *Checkpoint) save(artifactsDir string) error {
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(artifactsDir, checkpointFile), data, 0o644)
}

func (cp *Checkpoint) done(stage string) bool {
	return cp.Completed[stage]
}

func (cp *Checkpoint) mark(stage, artifactsDir string) error {
	cp.Completed[stage] = true
	return cp.save(artifactsDir)
}
