package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-cordova-archive/cordova"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/tools"
	"github.com/kballard/go-shellquote"
)

const (
	ipaPathEnvKey     = "BITRISE_IPA_PATH"
	appPathEnvKey     = "BITRISE_APP_PATH"
	dsymDirPathEnvKey = "BITRISE_DSYM_DIR_PATH"
	dsymZipPathEnvKey = "BITRISE_DSYM_PATH"
	apkPathEnvKey     = "BITRISE_APK_PATH"
)

// ConfigsModel ...
type ConfigsModel struct {
	WorkDir       string
	BuildConfig   string
	Platform      string
	Configuration string
	Target        string
	Options       string
	DeployDir     string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		WorkDir:       os.Getenv("workdir"),
		BuildConfig:   os.Getenv("build_config"),
		Platform:      os.Getenv("platform"),
		Configuration: os.Getenv("configuration"),
		Target:        os.Getenv("target"),
		Options:       os.Getenv("options"),
		DeployDir:     os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- WorkDir: %s", configs.WorkDir)
	log.Printf("- BuildConfig: %s", configs.BuildConfig)
	log.Printf("- Platform: %s", configs.Platform)
	log.Printf("- Configuration: %s", configs.Configuration)
	log.Printf("- Target: %s", configs.Target)
	log.Printf("- Options: %s", configs.Options)
	log.Printf("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if err := input.ValidateIfDirExists(configs.WorkDir); err != nil {
		return fmt.Errorf("WorkDir: %s", err)
	}

	if err := input.ValidateWithOptions(configs.Platform, "ios,android", "ios", "android"); err != nil {
		return fmt.Errorf("Platform: %s", err)
	}

	if err := input.ValidateIfNotEmpty(configs.Configuration); err != nil {
		return fmt.Errorf("Configuration: %s", err)
	}

	if err := input.ValidateIfNotEmpty(configs.Target); err != nil {
		return fmt.Errorf("Target: %s", err)
	}

	return nil
}

func zip(sourceDir, destinationZipPth string) error {
	parentDir := filepath.Dir(sourceDir)
	dirName := filepath.Base(sourceDir)
	cmd := command.New("/usr/bin/zip", "-rTy", destinationZipPth, dirName)
	cmd.SetDir(parentDir)
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to zip dir: %s, output: %s, error: %s", sourceDir, out, err)
	}

	return nil
}

func moveAndExportOutputs(outputs []string, deployDir, envKey string) (string, error) {
	outputToExport := ""
	for _, output := range outputs {
		outputFile, err := os.Open(output)
		if err != nil {
			return "", err
		}

		outputFileInfo, err := outputFile.Stat()
		if err != nil {
			return "", err
		}

		fileName := filepath.Base(output)
		destinationPth := filepath.Join(deployDir, fileName)

		if outputFileInfo.IsDir() {
			if err := command.CopyDir(output, destinationPth, false); err != nil {
				return "", err
			}
		} else {
			if err := command.CopyFile(output, destinationPth); err != nil {
				return "", err
			}
		}

		outputToExport = destinationPth
	}

	if outputToExport == "" {
		return "", nil
	}

	if err := tools.ExportEnvironmentWithEnvman(envKey, outputToExport); err != nil {
		return "", err
	}

	return outputToExport, nil
}

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		fail("Issue with input: %s", err)
	}

	// Fulfill cordova builder
	builder := cordova.New()

	platforms := []string{}
	if configs.Platform != "" {
		platformsSplit := strings.Split(configs.Platform, ",")
		for _, platform := range platformsSplit {
			platforms = append(platforms, strings.TrimSpace(platform))
		}

		builder.SetPlatforms(platforms...)
	}

	builder.SetConfiguration(configs.Configuration)
	builder.SetTarget(configs.Target)

	if configs.Options != "" {
		options, err := shellquote.Split(configs.Options)
		if err != nil {
			fail("Failed to shell split Options (%s), error: %s", configs.Options, err)
		}

		builder.SetCustomOptions(options...)
	}

	builder.SetBuildConfig(configs.BuildConfig)

	// Change dir to working directory
	workDir, err := pathutil.AbsPath(configs.WorkDir)
	if err != nil {
		fail("Failed to expand WorkDir (%s), error: %s", configs.WorkDir, err)
	}

	currentDir, err := pathutil.CurrentWorkingDirectoryAbsolutePath()
	if err != nil {
		fail("Failed to get current directory, error: %s", err)
	}

	if workDir != currentDir {
		fmt.Println()
		log.Infof("Switch working directory to: %s", workDir)

		revokeFunc, err := pathutil.RevokableChangeDir(workDir)
		if err != nil {
			fail("Failed to change working directory, error: %s", err)
		}
		defer func() {
			fmt.Println()
			log.Infof("Reset working directory")
			if err := revokeFunc(); err != nil {
				fail("Failed to reset working directory, error: %s", err)
			}
		}()
	}

	// cordova prepare
	fmt.Println()
	log.Infof("Preparing project")

	platformRemoveCmd := builder.PlatformCommand("rm")
	platformRemoveCmd.SetStdout(os.Stdout)
	platformRemoveCmd.SetStderr(os.Stderr)

	log.Donef("$ %s", platformRemoveCmd.PrintableCommandArgs())

	if err := platformRemoveCmd.Run(); err != nil {
		fail("cordova failed, error: %s", err)
	}

	platformAddCmd := builder.PlatformCommand("add")
	platformAddCmd.SetStdout(os.Stdout)
	platformAddCmd.SetStderr(os.Stderr)

	log.Donef("$ %s", platformAddCmd.PrintableCommandArgs())

	if err := platformAddCmd.Run(); err != nil {
		fail("cordova failed, error: %s", err)
	}

	// cordova build
	fmt.Println()
	log.Infof("Building project")

	buildCmd := builder.BuildCommand()
	buildCmd.SetStdout(os.Stdout)
	buildCmd.SetStderr(os.Stderr)

	log.Donef("$ %s", buildCmd.PrintableCommandArgs())

	if err := buildCmd.Run(); err != nil {
		fail("cordova failed, error: %s", err)
	}

	// collect outputs
	fmt.Println()
	log.Infof("Collecting outputs")

	iosOutputDir := filepath.Join(workDir, "platforms", "ios", "build", configs.Target)
	if exist, err := pathutil.IsDirExists(iosOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		ipaPattern := filepath.Join(iosOutputDir, "*.ipa")
		ipas, err := filepath.Glob(ipaPattern)
		if err != nil {
			fail("Failed to find ipas, with pattern (%s), error: %s", ipaPattern, err)
		}

		if len(ipas) > 0 {
			if exportedPth, err := moveAndExportOutputs(ipas, configs.DeployDir, ipaPathEnvKey); err != nil {
				fail("Failed to export ipas, error: %s", err)
			} else {
				log.Donef("The ipa path is now available in the Environment Variable: %s (value: %s)", ipaPathEnvKey, exportedPth)
			}
		}

		dsymPattern := filepath.Join(iosOutputDir, "*.dSYM")
		dsyms, err := filepath.Glob(dsymPattern)
		if err != nil {
			fail("Failed to find dSYMs, with pattern (%s), error: %s", dsymPattern, err)
		}

		if len(dsyms) > 0 {
			if exportedPth, err := moveAndExportOutputs(dsyms, configs.DeployDir, dsymDirPathEnvKey); err != nil {
				fail("Failed to export dsyms, error: %s", err)
			} else {
				log.Donef("The dsym dir path is now available in the Environment Variable: %s (value: %s)", dsymDirPathEnvKey, exportedPth)

				zippedExportedPth := exportedPth + ".zip"
				if err := zip(exportedPth, zippedExportedPth); err != nil {
					fail("Failed to zip dsym dir (%s), error: %s", exportedPth, err)
				}

				if err := tools.ExportEnvironmentWithEnvman(dsymZipPathEnvKey, zippedExportedPth); err != nil {
					fail("Failed to export dsym.zip (%s), error: %s", zippedExportedPth, err)
				}

				log.Donef("The dsym.zip path is now available in the Environment Variable: %s (value: %s)", dsymZipPathEnvKey, zippedExportedPth)
			}
		}

		appPattern := filepath.Join(iosOutputDir, "*.app")
		apps, err := filepath.Glob(appPattern)
		if err != nil {
			fail("Failed to find apps, with pattern (%s), error: %s", appPattern, err)
		}

		if len(apps) > 0 {
			if exportedPth, err := moveAndExportOutputs(apps, configs.DeployDir, appPathEnvKey); err != nil {
				fail("Failed to export apps, error: %s", err)
			} else {
				log.Donef("The app path is now available in the Environment Variable: %s (value: %s)", appPathEnvKey, exportedPth)
			}
		}
	}

	androidOutputDir := filepath.Join(workDir, "platforms", "android", "build", "outputs", "apk")
	if exist, err := pathutil.IsDirExists(androidOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		pattern := filepath.Join(androidOutputDir, "*.apk")
		apks, err := filepath.Glob(pattern)
		if err != nil {
			fail("Failed to find apks, with pattern (%s), error: %s", pattern, err)
		}

		if len(apks) > 0 {
			if exportedPth, err := moveAndExportOutputs(apks, configs.DeployDir, apkPathEnvKey); err != nil {
				fail("Failed to export apks, error: %s", err)
			} else {
				log.Donef("The apk path is now available in the Environment Variable: %s (value: %s)", apkPathEnvKey, exportedPth)
			}
		}
	}
}
