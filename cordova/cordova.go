package cordova

import (
	"fmt"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// Model ...
type Model struct {
	platforms      []string
	configuration  string
	target         string
	buildConfig    string
	customOptions  []string
	androidAppType string
}

// New ...
func New() *Model {
	return &Model{}
}

// SetPlatforms ...
func (builder *Model) SetPlatforms(platforms ...string) *Model {
	builder.platforms = platforms
	return builder
}

// SetConfiguration ...
func (builder *Model) SetConfiguration(configuration string) *Model {
	builder.configuration = configuration
	return builder
}

// SetTarget ...
func (builder *Model) SetTarget(target string) *Model {
	builder.target = target
	return builder
}

// SetBuildConfig ...
func (builder *Model) SetBuildConfig(buildConfig string) *Model {
	builder.buildConfig = buildConfig
	return builder
}

// SetAndroidAppType ...
// Possible app types: "apk", "aab"
func (builder *Model) SetAndroidAppType(appType string) *Model {
	builder.androidAppType = appType
	return builder
}

// SetCustomOptions ...
func (builder *Model) SetCustomOptions(customOptions ...string) *Model {
	builder.customOptions = customOptions
	return builder
}

func (builder *Model) commandSlice(cmd ...string) []string {
	cmdSlice := []string{"cordova"}
	cmdSlice = append(cmdSlice, cmd...)

	if len(cmd) == 1 && cmd[0] == "compile" {
		if builder.configuration != "" {
			cmdSlice = append(cmdSlice, "--"+builder.configuration)
		}
		if builder.target != "" {
			cmdSlice = append(cmdSlice, "--"+builder.target)
		}
	}

	if len(builder.platforms) > 0 {
		cmdSlice = append(cmdSlice, builder.platforms...)
	}

	if len(cmd) == 1 && cmd[0] == "compile" {
		if builder.buildConfig != "" {
			cmdSlice = append(cmdSlice, "--buildConfig", builder.buildConfig)
		}

		// Cordova CLI expects platform-specific arguments to be listed after a -- separator
		// We parse user-specified options and group them separately
		separator := "--"
		separatorIndex := sliceutil.IndexOfStringInSlice(separator, builder.customOptions)
		var generalOptions []string
		var platformOptions []string

		if builder.hasPlatformAndroid() {
			// Package type is platform-specific
			packageTypeValue := builder.androidAppType
			if packageTypeValue == "aab" {
				packageTypeValue = "bundle"
			}
			platformOptions = append(platformOptions, fmt.Sprintf("--packageType=%s", packageTypeValue))
		}

		for i, opt := range builder.customOptions {
			if opt == separator {
				continue
			}
			if separatorIndex >= 0 && i > separatorIndex {
				platformOptions = append(platformOptions, opt)
			} else {
				generalOptions = append(generalOptions, opt)
			}
		}

		cmdSlice = append(cmdSlice, generalOptions...)
		if len(platformOptions) > 0 {
			cmdSlice = append(cmdSlice, separator)
			cmdSlice = append(cmdSlice, platformOptions...)
		}

	}

	return cmdSlice
}

// PrepareCommand ...
func (builder *Model) PrepareCommand() *command.Model {
	cmdSlice := builder.commandSlice("prepare")
	return command.New(cmdSlice[0], cmdSlice[1:]...)
}

// CompileCommand ...
func (builder *Model) CompileCommand() *command.Model {
	cmdSlice := builder.commandSlice("compile")
	return command.New(cmdSlice[0], cmdSlice[1:]...)
}

func (builder *Model) hasPlatformAndroid() bool {
	for _, platform := range builder.platforms {
		if platform == "android" {
			return true
		}
	}
	return false
}
