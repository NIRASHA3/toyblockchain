package blockchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LoadState reads state from path. If the file does not exist, it returns a fresh chain.
func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewState(), nil
		}
		return State{}, fmt.Errorf("read state file %q: %w", path, err)
	}

	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode state file %q: %w", path, err)
	}
	if len(state.Chain) == 0 {
		return State{}, fmt.Errorf("decode state file %q: %w", path, ErrInvalidChain)
	}
	if state.Pending == nil {
		state.Pending = []Transaction{}
	}
	return state, nil
}

// ensureDir creates the directory for the given path if needed.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state directory %q: %w", dir, err)
	}
	return nil
}

// encodeTempFile writes state to a temporary file and returns the temp file and its name.
func encodeTempFile(dir string, state State) (*os.File, string, error) {
	tmp, err := os.CreateTemp(dir, ".toychain-*.tmp")
	if err != nil {
		return nil, "", fmt.Errorf("create temporary state file: %w", err)
	}
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		tmp.Close()
		return nil, tmp.Name(), fmt.Errorf("encode state: %w", err)
	}
	return tmp, tmp.Name(), nil
}

func appendRemoveTempFileError(err error, tmpName string, removeErr error) error {
	if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
		return err
	}
	if err == nil {
		return fmt.Errorf("remove temporary state file %q: %w", tmpName, removeErr)
	}
	return fmt.Errorf("%w; additionally remove temporary state file %q: %v", err, tmpName, removeErr)
}

// SaveState writes state atomically using a temporary file and rename.
func SaveState(path string, state State) (err error) {
	if path == "" {
		return fmt.Errorf("state file path is required")
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, tmpName, err := encodeTempFile(dir, state)
	if err != nil {
		os.Remove(tmpName)
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			err = appendRemoveTempFileError(err, tmpName, os.Remove(tmpName))
		}
	}()

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary state file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace state file %q: %w", path, err)
	}
	cleanup = false
	return nil
}
