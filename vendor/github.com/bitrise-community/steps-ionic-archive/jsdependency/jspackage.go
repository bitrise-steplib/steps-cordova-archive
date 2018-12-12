package jsdependency

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
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
func DetectTool(absPackageJSONDir string) (Tool, error) {
	if exist, err := pathutil.IsPathExists(filepath.Join(absPackageJSONDir, "yarn.lock")); err != nil {
		return Npm, fmt.Errorf("Failed to check if yarn.lock file exists in the workdir: %s", err)
	} else if exist {
		return Yarn, nil
	}
	return Npm, nil
}

// RemoveCommand returns command model to remove js dependencies
func RemoveCommand(packageManager Tool, commandScope CommandScope, pkg ...string) (*command.Model, error) {
	return createManagerCmd(packageManager,
		toolCommandBuilder(packageManager, removeCommand),
		commandScope,
		pkg...)
}

// AddCommand returns command model to install js dependencies
func AddCommand(packageManager Tool, commandScope CommandScope, pkg ...string) (*command.Model, error) {
	return createManagerCmd(packageManager,
		toolCommandBuilder(packageManager, addCommand),
		commandScope,
		pkg...)
}

// InstallGlobalDependencyCommand returns command model to install a global js dependency
func InstallGlobalDependencyCommand(packageManager Tool, dependency string, version string) ([]*command.Model, error) {
	if dependency == "" {
		return nil, errors.New("Dependency name unspecified")
	}
	var cmdSlice []*command.Model
	{
		cmd, err := RemoveCommand(packageManager, Local, dependency)
		if err != nil {
			return nil, err
		}
		cmdSlice = append(cmdSlice, cmd)
	}
	{
		cmd, err := AddCommand(packageManager, Global, dependency+"@"+version)
		if err != nil {
			return nil, err
		}
		cmdSlice = append(cmdSlice, cmd)
	}
	return cmdSlice, nil
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

func createManagerCmd(packageManager Tool, packageManagerCmd string, commandScope CommandScope, pkg ...string) (*command.Model, error) {
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
		return nil, fmt.Errorf("Command creation failed, error: %s", err)
	}
	return cmd, nil
}
