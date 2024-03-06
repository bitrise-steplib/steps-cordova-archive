package main

import (
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

func getIosOutputCandidateDirsPaths(workDir string, target string, configuration string) []string {
	targetPlatform := "iphonesimulator"
	if target == "device" {
		targetPlatform = "iphoneos"
	}

	// disable linting deprecated Title check SA1019
	cordovaIOS7targetComponent := strings.Title(configuration) + "-" + targetPlatform //nolint:staticcheck

	return []string{
		filepath.Join(workDir, "platforms", "ios", "build", target),                     // cordova-ios <7
		filepath.Join(workDir, "platforms", "ios", "build", cordovaIOS7targetComponent), // cordova-ios =>7
	}
}

func findFirstExistingDir(candidateDirPaths []string) string {
	for _, path := range candidateDirPaths {
		exist, err := pathutil.IsDirExists(path)
		if err != nil {
			log.Warnf("Failed to check if dir (%s) exist: %s", path, err)
			continue
		}

		if exist {
			return path
		}
	}

	return ""
}
