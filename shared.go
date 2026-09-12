package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type permissionStatus int

const (
	permissionUnknown permissionStatus = iota
	permissionGranted
	permissionDenied
)

func pathMissing(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	return false, fmt.Errorf("cannot tell if %s already exists: %v", path, err)
}

func writeFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot make %s directory: %v", dir, err)
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("error writing %s: %w", path, err)
	}
	slog.Debug("Wrote", "file", path)
	return nil
}

func extractFiles(embeddedFS embed.FS) (string, []fs.DirEntry, error) {
	dir := "."
	for {
		entries, err := fs.ReadDir(embeddedFS, dir)
		if err != nil {
			return "", nil, fmt.Errorf("cannot read the embedded config files in %s: %v", dir, err)
		}
		if len(entries) == 1 && entries[0].IsDir() {
			dir = path.Join(dir, entries[0].Name())
			continue
		}
		return dir, entries, nil
	}
}

// stagingSuffix is added to a directory name to get the place its new contents are built in.
const stagingSuffix = ".new"

func copyFS(embeddedFS embed.FS, targetDir string) error {
	sourceDir, entries, err := extractFiles(embeddedFS)
	if err != nil {
		return err
	}

	staging := targetDir + stagingSuffix
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("cannot remove %s: %v", staging, err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("cannot make %s directory: %v", staging, err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	for _, entry := range entries {
		if entry.IsDir() {
			panic(fmt.Sprintf("%s holds the directory %s, but copying puts every file straight into one directory", sourceDir, entry.Name()))
		}

		data, err := fs.ReadFile(embeddedFS, path.Join(sourceDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("cannot read the embedded file %s: %v", entry.Name(), err)
		}
		dstPath := filepath.Join(staging, entry.Name())
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return fmt.Errorf("failed writing omacy file %s: %v", dstPath, err)
		}
		slog.Debug("Created", "file", filepath.Join(targetDir, entry.Name()))
	}

	return replaceDir(staging, targetDir)
}

// replaceDir puts staging where target is in a single step. Anything watching target sees the old
// files or the new ones, never a directory that is missing or half written.
func replaceDir(staging, target string) error {
	missing, err := pathMissing(target)
	if err != nil {
		return err
	}
	if missing {
		parent := filepath.Dir(target)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("cannot make %s directory: %v", parent, err)
		}
		if err := os.Rename(staging, target); err != nil {
			return fmt.Errorf("cannot move %s to %s: %v", staging, target, err)
		}
		return nil
	}
	if err := unix.RenameatxNp(unix.AT_FDCWD, staging, unix.AT_FDCWD, target, unix.RENAME_SWAP); err != nil {
		return fmt.Errorf("cannot swap %s into %s: %v", staging, target, err)
	}
	return nil
}
