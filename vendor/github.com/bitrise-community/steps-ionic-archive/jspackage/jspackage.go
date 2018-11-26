package jspackage

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

// Manager identifies a package manager tool
type Manager string

// Package manager types
const (
	Npm  Manager = "npm"
	Yarn Manager = "yarn"
)

// DetectManager returns the Js package manager used, e.g. npm or yarn
func DetectManager(absPackageJSONDir string) Manager {
	if exist, err := pathutil.IsPathExists(filepath.Join(absPackageJSONDir, "yarn.lock")); err != nil {
		log.Warnf("Failed to check if yarn.lock file exists in the workdir: %s", err)
		return Npm
	} else if exist {
		return Yarn
	}
	return Npm
}

// Remove removes installed js dependencies using the selected package manager
func Remove(packageManager Manager, isGlobal bool, pkg ...string) error {
	var cmd *command.Model
	switch packageManager {
	case Npm:
		args := []string{"remove"}
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
		args = append(args, "remove")
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

// Add installs js dependencies using the selected package manager
func Add(packageManager Manager, isGlobal bool, pkg ...string) error {
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
