package jsdependency

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

// Tool identifies a package manager tool
type Tool string

// Package manager types
const (
	Npm  Tool = "npm"
	Yarn Tool = "yarn"
)

// CommandScope describes package manager command scope (global, or not)
type CommandScope string

// PackageScope types
const (
	Local  CommandScope = "local"
	Global CommandScope = "global"
)

type managerCommand string

const (
	addCommand    managerCommand = "add"
	removeCommand managerCommand = "remove"
)

// DetectTool returns the Js package manager used, e.g. npm or yarn
func DetectTool(absPackageJSONDir string) Tool {
	if exist, err := pathutil.IsPathExists(filepath.Join(absPackageJSONDir, "yarn.lock")); err != nil {
		log.Warnf("Failed to check if yarn.lock file exists in the workdir: %s", err)
		return Npm
	} else if exist {
		return Yarn
	}
	return Npm
}

// Remove removes installed js dependencies using the selected package manager
func Remove(packageManager Tool, commandScope CommandScope, pkg ...string) error {
	return runManagerCmd(packageManager,
		toolCommandBuilder(packageManager, removeCommand),
		commandScope,
		pkg...)
}

// Add installs js dependencies using the selected package manager
func Add(packageManager Tool, commandScope CommandScope, pkg ...string) error {
	return runManagerCmd(packageManager,
		toolCommandBuilder(packageManager, addCommand),
		commandScope,
		pkg...)
}

func toolCommandBuilder(packageManger Tool, command managerCommand) string {
	if command == removeCommand {
		return "remove"
	}
	// Add command
	if packageManger == Npm {
		return "install"
	}
	return "add"
}

func runManagerCmd(packageManager Tool, packageManagerCmd string, commandScope CommandScope, pkg ...string) error {
	var commandArgs []string
	switch packageManager {
	case Npm:
		commandArgs = []string{"npm", packageManagerCmd}
		if commandScope == Global {
			commandArgs = append(commandArgs, "-g")
		}
		commandArgs = append(commandArgs, pkg...)
	case Yarn:
		commandArgs = []string{"yarn"}
		if commandScope == Global {
			commandArgs = append(commandArgs, "global")
		}
		commandArgs = append(commandArgs, packageManagerCmd)
		commandArgs = append(commandArgs, pkg...)
	}
	cmd, err := command.NewFromSlice(commandArgs)
	if err != nil {
		return fmt.Errorf("Command creation failed, error: %s", err)
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
