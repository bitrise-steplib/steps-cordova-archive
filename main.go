package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-community/steps-cordova-archive/cordova"
	"github.com/bitrise-community/steps-ionic-archive/jspackage"
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
	Platform       string
	Configuration  string
	Target         string
	BuildConfig    string
	AddPlatform    string
	ReAddPlatform  string
	CordovaVersion string
	WorkDir        string
	Options        string
	DeployDir      string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		Platform:       os.Getenv("platform"),
		Configuration:  os.Getenv("configuration"),
		Target:         os.Getenv("target"),
		BuildConfig:    os.Getenv("build_config"),
		AddPlatform:    os.Getenv("add_platform"),
		ReAddPlatform:  os.Getenv("readd_platform"),
		CordovaVersion: os.Getenv("cordova_version"),
		WorkDir:        os.Getenv("workdir"),
		Options:        os.Getenv("options"),
		DeployDir:      os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- Platform: %s", configs.Platform)
	log.Printf("- Configuration: %s", configs.Configuration)
	log.Printf("- Target: %s", configs.Target)
	log.Printf("- BuildConfig: %s", configs.BuildConfig)
	log.Printf("- AddPlatform: %s", configs.AddPlatform)
	log.Printf("- ReAddPlatform: %s", configs.ReAddPlatform)
	log.Printf("- CordovaVersion: %s", configs.CordovaVersion)
	log.Printf("- WorkDir: %s", configs.WorkDir)
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

	if err := input.ValidateWithOptions(configs.AddPlatform, "true", "false"); err != nil {
		return fmt.Errorf("AddPlatform: %s", err)
	}

	if err := input.ValidateWithOptions(configs.ReAddPlatform, "true", "false"); err != nil {
		return fmt.Errorf("ReAddPlatform: %s", err)
	}

	if err := input.ValidateIfNotEmpty(configs.Configuration); err != nil {
		return fmt.Errorf("Configuration: %s", err)
	}

	if err := input.ValidateIfNotEmpty(configs.Target); err != nil {
		return fmt.Errorf("Target: %s", err)
	}

	return nil
}

func moveAndExportOutputs(outputs []string, deployDir, envKey string, isOnlyContent bool) (string, error) {
	outputToExport := ""
	for _, output := range outputs {
		info, err := os.Lstat(output)
		if err != nil {
			return "", err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolvedPth, err := os.Readlink(output)
			if err != nil {
				return "", err
			}

			if exist, err := pathutil.IsPathExists(resolvedPth); err != nil {
				return "", err
			} else if !exist {
				return "", fmt.Errorf("resolved path: %s does not exist", resolvedPth)
			}

			resolvedInfo, err := os.Lstat(resolvedPth)
			if err != nil {
				return "", err
			}

			if resolvedInfo.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("resolved path: %s is still symlink", resolvedPth)
			}

			output = resolvedPth
			info = resolvedInfo
		}

		fileName := filepath.Base(output)
		destinationPth := filepath.Join(deployDir, fileName)

		if info.IsDir() {
			if err := command.CopyDir(output, destinationPth, isOnlyContent); err != nil {
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

func findArtifact(rootDir, ext string, buildStart time.Time) ([]string, error) {
	var matches []string
	if walkErr := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if info.ModTime().Before(buildStart) {
			return nil
		}

		if filepath.Ext(path) == "."+ext {
			matches = append(matches, path)
		}
		return err
	}); walkErr != nil {
		return nil, walkErr
	}
	return matches, nil
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

	// Change dir to working directory
	workDir, err := pathutil.AbsPath(configs.WorkDir)
	log.Debugf("New work dir: %s", workDir)
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

	// Update cordova version
	if configs.CordovaVersion != "" {
		fmt.Println()
		log.Infof("Updating cordova version to: %s", configs.CordovaVersion)
		packageName := "cordova"
		packageName += "@" + configs.CordovaVersion

		packageManager := jspackage.DetectManager(workDir)
		log.TPrintf("Js package manager used: %s", packageManager)
		if err := jspackage.Add(packageManager, true, packageName); err != nil {
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

	// cordova prepare
	fmt.Println()
	log.Infof("Preparing project")

	if configs.AddPlatform == "true" {
		if configs.ReAddPlatform == "true" {
			platformRemoveCmd := builder.PlatformCommand("rm")
			platformRemoveCmd.SetStdout(os.Stdout)
			platformRemoveCmd.SetStderr(os.Stderr)

			log.Donef("$ %s", platformRemoveCmd.PrintableCommandArgs())

			if err := platformRemoveCmd.Run(); err != nil {
				fail("cordova remove platform failed, error: %s", err)
			}
		}

		platformAddCmd := builder.PlatformCommand("add")
		platformAddCmd.SetStdout(os.Stdout)
		platformAddCmd.SetStderr(os.Stderr)

		log.Donef("$ %s", platformAddCmd.PrintableCommandArgs())

		if err := platformAddCmd.Run(); err != nil {
			fail("cordova add platform failed, error: %s", err)
		}
	} else {
		platformPrepareCmd := builder.PlatformCommand("prepare")
		platformPrepareCmd.SetStdout(os.Stdout)
		platformPrepareCmd.SetStderr(os.Stderr)

		log.Donef("$ %s", platformPrepareCmd.PrintableCommandArgs())

		if err := platformPrepareCmd.Run(); err != nil {
			fail("cordova prepare platform failed, error: %s", err)
		}
	}

	// cordova build
	fmt.Println()
	log.Infof("Building project")

	buildCmd := builder.CompileCommand()
	buildCmd.SetStdout(os.Stdout)
	buildCmd.SetStderr(os.Stderr)

	log.Donef("$ %s", buildCmd.PrintableCommandArgs())

	compileStart := time.Now()

	if err := buildCmd.Run(); err != nil {
		fail("cordova build failed, error: %s", err)
	}

	// collect outputs
	iosOutputDirExist := false
	iosOutputDir := filepath.Join(workDir, "platforms", "ios", "build", configs.Target)
	if exist, err := pathutil.IsDirExists(iosOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		iosOutputDirExist = true

		fmt.Println()
		log.Infof("Collecting ios outputs")

		ipas, err := findArtifact(iosOutputDir, "ipa", compileStart)
		if err != nil {
			fail("Failed to find ipas in dir (%s), error: %s", iosOutputDir, err)
		}

		if len(ipas) > 0 {
			if exportedPth, err := moveAndExportOutputs(ipas, configs.DeployDir, ipaPathEnvKey, false); err != nil {
				fail("Failed to export ipas, error: %s", err)
			} else {
				log.Donef("The ipa path is now available in the Environment Variable: %s (value: %s)", ipaPathEnvKey, exportedPth)
			}
		}

		dsyms, err := findArtifact(iosOutputDir, "dSYM", compileStart)
		if err != nil {
			fail("Failed to find dSYMs in dir (%s), error: %s", iosOutputDir, err)
		}

		if len(dsyms) > 0 {
			if exportedPth, err := moveAndExportOutputs(dsyms, configs.DeployDir, dsymDirPathEnvKey, true); err != nil {
				fail("Failed to export dsyms, error: %s", err)
			} else {
				log.Donef("The dsym dir path is now available in the Environment Variable: %s (value: %s)", dsymDirPathEnvKey, exportedPth)

				zippedExportedPth := exportedPth + ".zip"
				if err := ziputil.ZipDir(exportedPth, zippedExportedPth, false); err != nil {
					fail("Failed to zip dsym dir (%s), error: %s", exportedPth, err)
				}

				if err := tools.ExportEnvironmentWithEnvman(dsymZipPathEnvKey, zippedExportedPth); err != nil {
					fail("Failed to export dsym.zip (%s), error: %s", zippedExportedPth, err)
				}

				log.Donef("The dsym.zip path is now available in the Environment Variable: %s (value: %s)", dsymZipPathEnvKey, zippedExportedPth)
			}
		}

		apps, err := findArtifact(iosOutputDir, "app", compileStart)
		if err != nil {
			fail("Failed to find apps in dir (%s), error: %s", iosOutputDir, err)
		}

		if len(apps) > 0 {
			if exportedPth, err := moveAndExportOutputs(apps, configs.DeployDir, appDirPathEnvKey, true); err != nil {
				log.Warnf("Failed to export apps, error: %s", err)
			} else {
				log.Donef("The app dir path is now available in the Environment Variable: %s (value: %s)", appDirPathEnvKey, exportedPth)

				zippedExportedPth := exportedPth + ".zip"
				if err := ziputil.ZipDir(exportedPth, zippedExportedPth, false); err != nil {
					fail("Failed to zip app dir (%s), error: %s", exportedPth, err)
				}

				if err := tools.ExportEnvironmentWithEnvman(appZipPathEnvKey, zippedExportedPth); err != nil {
					fail("Failed to export app.zip (%s), error: %s", zippedExportedPth, err)
				}

				log.Donef("The app.zip path is now available in the Environment Variable: %s (value: %s)", appZipPathEnvKey, zippedExportedPth)
			}
		}
	}

	androidOutputDirExist := false
	// examples for apk paths:
	// PROJECT_ROOT/platforms/android/app/build/outputs/apk/debug/app-debug.apk
	// PROJECT_ROOT/platforms/android/build/outputs/apk/debug/app-debug.apk
	androidOutputDir := filepath.Join(workDir, "platforms", "android")
	if exist, err := pathutil.IsDirExists(androidOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		androidOutputDirExist = true

		fmt.Println()
		log.Infof("Collecting android outputs")

		apks, err := findArtifact(androidOutputDir, "apk", compileStart)
		if err != nil {
			fail("Failed to find apks in dir (%s), error: %s", androidOutputDir, err)
		}

		if len(apks) > 0 {
			if exportedPth, err := moveAndExportOutputs(apks, configs.DeployDir, apkPathEnvKey, false); err != nil {
				fail("Failed to export apks, error: %s", err)
			} else {
				log.Donef("The apk path is now available in the Environment Variable: %s (value: %s)", apkPathEnvKey, exportedPth)
			}
		}
	}

	if !iosOutputDirExist && !androidOutputDirExist {
		log.Warnf("No ios nor android platform's output dir exist")
		fail("No output generated")
	}
}
