package main

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/cache"
	"github.com/bitrise-io/go-utils/pathutil"
)

// cacheNpm marks node_modules for caching
func cacheNpm(workdir string) error {
	localPackageDir := filepath.Join(workdir, "node_modules")

	switch exist, err := pathutil.IsDirExists(localPackageDir); {
	case err != nil:
		return fmt.Errorf("failed to check directory existence, error: %s", err)
	case !exist:
		return fmt.Errorf("local node_modules directory does not exist: %s", localPackageDir)
	}

	npmCache := cache.New()

	// cache update indicator (package-lock.json)
	// is not used at the moment as it cache-push performed slower with it
	npmCache.IncludePath(localPackageDir)

	if err := npmCache.Commit(); err != nil {
		return fmt.Errorf("failed to mark node_modules directory to be cached, error: %s", err)
	}

	return nil
}
