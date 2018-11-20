package main

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

// JsPackageManager describes if npm or yarn is used
type JsPackageManager int

// JsPackageManager types
const (
	Npm  JsPackageManager = 0
	Yarn JsPackageManager = 1
)

func detectJsPackageManager(absPackageJSONDir string) JsPackageManager {
	if exist, err := pathutil.IsPathExists(filepath.Join(absPackageJSONDir, "yarn.lock")); err != nil {
		log.Warnf("Failed to check if yarn.lock file exists in the workdir: %s", err)
		log.TPrintf("Dependency manager: npm")
		return Npm
	} else if exist {
		log.TPrintf("Dependency manager: yarn")
		return Yarn
	} else {
		log.TPrintf("Dependency manager: npm")
		return Npm
	}
}

func addJsPackages(packageManager JsPackageManager, isGlobal bool, pkg ...string) error {
	var cmd *command.Model
	switch packageManager {
	case Npm:
		args := []string{"install"}
		if isGlobal {
			args = append(args, "-g")
		}
		args = append(args, pkg...)
		cmd = command.New("npm", args...)
	case Yarn:
		args := []string{}
		if isGlobal {
			args = append(args, "global")
		}
		args = append(args, "add")
		args = append(args, pkg...)
		cmd = command.New("yarn", args...)
	}
	fmt.Println()
	log.Donef("$ %s", cmd.PrintableCommandArgs())
	fmt.Println()

	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		if errorutil.IsExitStatusError(err) {
			return fmt.Errorf("%s failed, output: %s", cmd.PrintableCommandArgs(), out)
		}
		return fmt.Errorf("%s failed, error: %s", cmd.PrintableCommandArgs(), err)
	}
	return nil
}
