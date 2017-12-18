package cordova

import "github.com/bitrise-io/go-utils/command"

// Model ...
type Model struct {
	platforms     []string
	configuration string
	target        string
	buildConfig   string
	customOptions []string
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

// SetCustomOptions ...
func (builder *Model) SetCustomOptions(customOptions ...string) *Model {
	builder.customOptions = customOptions
	return builder
}

func (builder *Model) commandSlice(cmd ...string) []string {
	cmdSlice := []string{"cordova"}
	cmdSlice = append(cmdSlice, cmd...)

	if len(cmd) == 1 && cmd[0] == "build" {
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

	if len(cmd) == 1 && cmd[0] == "build" {
		if builder.buildConfig != "" {
			cmdSlice = append(cmdSlice, "--buildConfig", builder.buildConfig)
		}
		cmdSlice = append(cmdSlice, builder.customOptions...)
	}

	return cmdSlice
}

// PlatformCommand ...
func (builder *Model) PlatformCommand(cmd string) *command.Model {
	cmdSlice := builder.commandSlice("platform", cmd)
	return command.New(cmdSlice[0], cmdSlice[1:]...)
}

// BuildCommand ...
func (builder *Model) BuildCommand() *command.Model {
	cmdSlice := builder.commandSlice("build")
	return command.New(cmdSlice[0], cmdSlice[1:]...)
}
