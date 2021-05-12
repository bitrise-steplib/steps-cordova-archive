package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-steputils/jsdependency"
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-steplib/steps-cordova-archive/cordova"
	"github.com/kballard/go-shellquote"
)

const (
	ipaPathEnvKey = "BITRISE_IPA_PATH"

	appZipPathEnvKey = "BITRISE_APP_PATH"
	appDirPathEnvKey = "BITRISE_APP_DIR_PATH"

	dsymDirPathEnvKey = "BITRISE_DSYM_DIR_PATH"
	dsymZipPathEnvKey = "BITRISE_DSYM_PATH"

	apkPathEnvKey = "BITRISE_APK_PATH"
	aabPathEnvKey = "BITRISE_AAB_PATH"
)

type config struct {
	Platform       string `env:"platform,opt['ios,android',ios,android]"`
	Configuration  string `env:"configuration,required"`
	Target         string `env:"target,required"`
	BuildConfig    string `env:"build_config"`
	RunPrepare     bool   `env:"run_cordova_prepare,opt[true,false]"`
	CordovaVersion string `env:"cordova_version"`
	WorkDir        string `env:"workdir,dir"`
	Options        string `env:"options"`
	DeployDir      string `env:"BITRISE_DEPLOY_DIR"`
	UseCache       bool   `env:"cache_local_deps,opt[true,false]"`
}

func installDependency(packageManager jsdependency.Tool, name string, version string) error {
	cmdSlice, err := jsdependency.InstallGlobalDependencyCommand(packageManager, name, version)
	if err != nil {
		return fmt.Errorf("Failed to update %s version, error: %s", name, err)
	}
	for i, cmd := range cmdSlice {
		fmt.Println()
		log.Donef("$ %s", cmd.Slice.PrintableCommandArgs())
		fmt.Println()

		// Yarn returns an error if the package is not added before removal, ignoring
		if out, err := cmd.Slice.RunAndReturnTrimmedCombinedOutput(); err != nil && !(packageManager == jsdependency.Yarn && i == 0) {
			if errorutil.IsExitStatusError(err) {
				return fmt.Errorf("Failed to update %s version: %s failed, output: %s", name, cmd.Slice.PrintableCommandArgs(), out)
			}
			return fmt.Errorf("Failed to update %s version: %s failed, error: %s", name, cmd.Slice.PrintableCommandArgs(), err)
		}
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

func checkBuildProducts(apks []string, aabs []string, apps []string, ipas []string, platforms []string, target string) error {
	// if android in platforms
	if sliceutil.IsStringInSlice("android", platforms) {
		if len(apks) == 0 && len(aabs) == 0 {
			return errors.New("no apk or aab generated")
		}
	}
	// if ios in platforms
	if sliceutil.IsStringInSlice("ios", platforms) {
		if len(apps) == 0 && target == "emulator" {
			return errors.New("No app generated")
		}
		if len(ipas) == 0 && target == "device" {
			return errors.New("no ipa generated")
		}
	}
	return nil
}

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	var configs config
	if err := stepconf.Parse(&configs); err != nil {
		fail("Could not create config: %s", err)
	}
	fmt.Println()
	stepconf.Print(configs)

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
		log.Printf("\n")
		log.Infof("Updating cordova version to: %s", configs.CordovaVersion)

		packageManager, err := jsdependency.DetectTool(workDir)
		if err != nil {
			log.Warnf("%s", err)
		}
		log.Printf("Js package manager used: %s", packageManager)

		if err := installDependency(packageManager, "cordova", configs.CordovaVersion); err != nil {
			fail("Updating cordova failed, error: %s", err)
		}
	}

	// Print cordova and ionic version
	cordovaVersion, err := cordova.CurrentVersion()
	if err != nil {
		fail(err.Error())
	}

	fmt.Println()
	log.Printf("Using cordova version:\n%s", colorstring.Green(cordovaVersion))

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
	if configs.RunPrepare {
		fmt.Println()
		log.Infof("Preparing project")
		platformPrepareCmd := builder.PrepareCommand()
		platformPrepareCmd.SetStdout(os.Stdout).SetStderr(os.Stderr)
		log.Donef("$ %s", platformPrepareCmd.PrintableCommandArgs())

		if err := platformPrepareCmd.Run(); err != nil {
			fail("cordova prepare failed, error: %s", err)
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
	var ipas, apps []string
	iosOutputDirExist := false
	iosOutputDir := filepath.Join(workDir, "platforms", "ios", "build", configs.Target)
	if exist, err := pathutil.IsDirExists(iosOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		iosOutputDirExist = true

		fmt.Println()
		log.Infof("Collecting ios outputs")

		ipas, err = findArtifact(iosOutputDir, "ipa", compileStart)
		if err != nil {
			fail("Failed to find ipas in dir (%s), error: %s", iosOutputDir, err)
		}

		if configs.Target == "device" && len(ipas) > 0 {
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

		apps, err = findArtifact(iosOutputDir, "app", compileStart)
		if err != nil {
			fail("Failed to find apps in dir (%s), error: %s", iosOutputDir, err)
		}

		if configs.Target == "emulator" && len(apps) > 0 {
			if exportedPth, err := moveAndExportOutputs(apps, configs.DeployDir, appDirPathEnvKey, true); err != nil {
				fail("Failed to export apps, error: %s", err)
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

	var apks, aabs []string
	androidOutputDirExist := false
	// examples for apk paths:
	// PROJECT_ROOT/platforms/android/app/build/outputs/apk/debug/app-debug.apk
	// PROJECT_ROOT/platforms/android/build/outputs/apk/debug/app-debug.apk
	// PROJECT_ROOT/platforms/android/build/outputs/bundle/release/app.aab
	androidOutputDir := filepath.Join(workDir, "platforms", "android")
	if exist, err := pathutil.IsDirExists(androidOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", androidOutputDir, err)
	} else if exist {
		androidOutputDirExist = true

		fmt.Println()
		log.Infof("Collecting android outputs")

		apks, err = findArtifact(androidOutputDir, "apk", compileStart)
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

		aabs, err = findArtifact(androidOutputDir, "aab", compileStart)
		if err != nil {
			fail("Failed to find aab in dir (%s), error: %s", androidOutputDir, err)
		}

		if len(aabs) > 0 {
			if exportedPth, err := moveAndExportOutputs(aabs, configs.DeployDir, aabPathEnvKey, false); err != nil {
				fail("Failed to export aabs, error: %s", err)
			} else {
				log.Donef("The aab path is now available in the Environment Variable: %s (value: %s)", aabPathEnvKey, exportedPth)
			}
		}
	}

	if !iosOutputDirExist && !androidOutputDirExist {
		log.Warnf("No ios nor android platform's output dir exist")
		fail("No output generated")
	}

	if err := checkBuildProducts(apks, aabs, apps, ipas, platforms, configs.Target); err != nil {
		fail("Build outputs missing: %s", err)
	}

	if configs.UseCache {
		if err := cacheNpm(workDir); err != nil {
			log.Warnf("Failed to mark files for caching, error: %s", err)
		}
	}
}
