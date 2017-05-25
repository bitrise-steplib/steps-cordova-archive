package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-community/steps-cordova-archive/cordova"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/tools"
	"github.com/kballard/go-shellquote"
)

const (
	ipaPathEnvKey = "BITRISE_IPA_PATH"

	appZipPathEnvKey = "BITRISE_APP_PATH"
	appDirPathEnvKey = "BITRISE_APP_DIR_PATH"

	dsymDirPathEnvKey = "BITRISE_DSYM_DIR_PATH"
	dsymZipPathEnvKey = "BITRISE_DSYM_PATH"

	apkPathEnvKey = "BITRISE_APK_PATH"
)

// ConfigsModel ...
type ConfigsModel struct {
	WorkDir        string
	BuildConfig    string
	Platform       string
	Configuration  string
	Target         string
	CordovaVersion string
	Options        string
	DeployDir      string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		WorkDir:        os.Getenv("workdir"),
		BuildConfig:    os.Getenv("build_config"),
		Platform:       os.Getenv("platform"),
		Configuration:  os.Getenv("configuration"),
		Target:         os.Getenv("target"),
		CordovaVersion: os.Getenv("cordova_version"),
		Options:        os.Getenv("options"),
		DeployDir:      os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- WorkDir: %s", configs.WorkDir)
	log.Printf("- BuildConfig: %s", configs.BuildConfig)
	log.Printf("- Platform: %s", configs.Platform)
	log.Printf("- Configuration: %s", configs.Configuration)
	log.Printf("- Target: %s", configs.Target)
	log.Printf("- CordovaVersion: %s", configs.CordovaVersion)
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

func npmUpdate(tool, version string) error {
	args := []string{}
	if version == "latest" {
		args = append(args, "install", "-g", tool)
	} else {
		args = append(args, "install", "-g", tool+"@"+version)
	}

	cmd := command.New("npm", args...)

	log.Donef("$ %s", cmd.PrintableCommandArgs())

	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		return fmt.Errorf("command failed, output: %s, error: %s", out, err)
	}
	return nil
}

func toolVersion(tool string) (string, error) {
	out, err := command.New(tool, "-v").RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return "", fmt.Errorf("$ %s -v failed, output: %s, error: %s", tool, out, err)
	}

	lines := strings.Split(out, "\n")
	if len(lines) > 0 {
		return lines[len(lines)-1], nil
	}

	return out, nil
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

	// Update cordova version
	if configs.CordovaVersion != "" {
		fmt.Println()
		log.Infof("Updating cordova version to: %s", configs.CordovaVersion)

		if err := npmUpdate("cordova", configs.CordovaVersion); err != nil {
			fail(err.Error())
		}
	}

	// Print cordova and ionic version
	cordovaVersion, err := toolVersion("cordova")
	if err != nil {
		fail(err.Error())
	}

	fmt.Println()
	log.Printf("using cordova version:\n%s", colorstring.Green(cordovaVersion))

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

	iosOutputDir := filepath.Join(workDir, "platforms", "ios", "build", configs.Target)
	if exist, err := pathutil.IsDirExists(iosOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		fmt.Println()
		log.Infof("Collecting ios outputs")

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
				if err := ziputil.Zip(exportedPth, zippedExportedPth); err != nil {
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
			if exportedPth, err := moveAndExportOutputs(apps, configs.DeployDir, appDirPathEnvKey); err != nil {
				fail("Failed to export apps, error: %s", err)
			} else {
				log.Donef("The app dir path is now available in the Environment Variable: %s (value: %s)", appDirPathEnvKey, exportedPth)

				zippedExportedPth := exportedPth + ".zip"
				if err := ziputil.Zip(exportedPth, zippedExportedPth); err != nil {
					fail("Failed to zip app dir (%s), error: %s", exportedPth, err)
				}

				if err := tools.ExportEnvironmentWithEnvman(appZipPathEnvKey, zippedExportedPth); err != nil {
					fail("Failed to export app.zip (%s), error: %s", zippedExportedPth, err)
				}

				log.Donef("The app.zip path is now available in the Environment Variable: %s (value: %s)", appZipPathEnvKey, zippedExportedPth)
			}
		}
	}

	androidOutputDir := filepath.Join(workDir, "platforms", "android", "build", "outputs", "apk")
	if exist, err := pathutil.IsDirExists(androidOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		fmt.Println()
		log.Infof("Collecting android outputs")

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
