package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bitrise-community/steps-ionic-archive/ionic"
	"github.com/bitrise-community/steps-ionic-archive/jsdependency"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"github.com/bitrise-tools/go-steputils/tools"
	ver "github.com/hashicorp/go-version"
	shellquote "github.com/kballard/go-shellquote"
)

const (
	ipaPathEnvKey = "BITRISE_IPA_PATH"

	appZipPathEnvKey = "BITRISE_APP_PATH"
	appDirPathEnvKey = "BITRISE_APP_DIR_PATH"

	dsymDirPathEnvKey = "BITRISE_DSYM_DIR_PATH"
	dsymZipPathEnvKey = "BITRISE_DSYM_PATH"

	apkPathEnvKey     = "BITRISE_APK_PATH"
	apkPathListEnvKey = "BITRISE_APK_PATH_LIST"
)

type config struct {
	Platform      string `env:"platform,opt['ios,android',ios,android]"`
	Configuration string `env:"configuration,required"`
	Target        string `env:"target,required"`
	BuildConfig   string `env:"build_config"`
	Options       string `env:"options"`

	Username string `env:"ionic_username"`
	Password string `env:"ionic_password"`

	RunPrepare     bool   `env:"run_ionic_prepare,opt[true,false]"`
	IonicVersion   string `env:"ionic_version"`
	CordovaVersion string `env:"cordova_version"`

	WorkDir   string `env:"workdir,dir"`
	DeployDir string `env:"BITRISE_DEPLOY_DIR"`
}

func installDependency(packageManager jsdependency.Tool, name string, version string) error {
	fmt.Println()
	log.Infof("Updating %s version to: %s", name, version)
	cmdSlice, err := jsdependency.InstallGlobalDependencyCommand(packageManager, name, version)
	if err != nil {
		return fmt.Errorf("Failed to update %s version, error: %s", name, err)
	}
	for i, cmd := range cmdSlice {
		fmt.Println()
		log.Donef("$ %s", cmd.PrintableCommandArgs())
		fmt.Println()

		// Yarn returns an error if the package is not added before removal, ignoring
		if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil && !(packageManager == jsdependency.Yarn && i == 0) {
			if errorutil.IsExitStatusError(err) {
				return fmt.Errorf("Failed to update %s version: %s failed, output: %s", name, cmd.PrintableCommandArgs(), out)
			}
			return fmt.Errorf("Failed to update %s version: %s failed, error: %s", name, cmd.PrintableCommandArgs(), err)
		}
	}
	return nil
}

func moveAndExportOutputs(outputs []string, deployDir, envKey string, envListKey string) (string, []string, error) {
	outputToExport := ""
	APKPaths := make([]string, len(outputs))
	for x, output := range outputs {
		info, err := os.Lstat(output)
		if err != nil {
			return "", []string{}, err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolvedPth, err := os.Readlink(output)
			if err != nil {
				return "", []string{}, err
			}

			log.Warnf("Output: %s is a symlink to: %s", output, resolvedPth)

			if exist, err := pathutil.IsPathExists(resolvedPth); err != nil {
				return "", []string{}, err
			} else if !exist {
				return "", []string{}, fmt.Errorf("resolved path: %s does not exist", resolvedPth)
			}

			resolvedInfo, err := os.Lstat(resolvedPth)
			if err != nil {
				return "", []string{}, err
			}

			if resolvedInfo.Mode()&os.ModeSymlink != 0 {
				return "", []string{}, fmt.Errorf("resolved path: %s is still symlink", resolvedPth)
			}

			output = resolvedPth
			info = resolvedInfo
		}

		fileName := filepath.Base(output)
		destinationPth := filepath.Join(deployDir, fileName)

		if info.IsDir() {
			if err := command.CopyDir(output, destinationPth, false); err != nil {
				return "", []string{}, err
			}
		} else {
			if err := command.CopyFile(output, destinationPth); err != nil {
				return "", []string{}, err
			}
		}

		outputToExport = destinationPth
		APKPaths[x] = destinationPth
	}

	if outputToExport == "" {
		return "", []string{}, nil
	}

	if err := tools.ExportEnvironmentWithEnvman(envKey, outputToExport); err != nil {
		return "", []string{}, err
	}

	if err := tools.ExportEnvironmentWithEnvman(envListKey, strings.Join(APKPaths, "|")); err != nil {
		return "", []string{}, err
	}

	return outputToExport, APKPaths, nil
}

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func findArtifact(dir, ext string, buildStart time.Time) ([]string, error) {
	var matches []string
	if walkErr := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if fi.ModTime().Before(buildStart) {
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

func main() {
	// Parse inputs
	var configs config
	if err := stepconf.Parse(&configs); err != nil {
		fail("Could not create config: %s", err)
	}
	fmt.Println()
	stepconf.Print(configs)

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

	// Update cordova and ionic version
	packageManager, err := jsdependency.DetectTool(workDir)
	if err != nil {
		log.Warnf("%s", err)
	}
	log.Printf("Js package manager used: %s", packageManager)
	if configs.CordovaVersion != "" {
		if err := installDependency(packageManager, "cordova", configs.CordovaVersion); err != nil {
			fail("%s", err)
		}
	}
	if configs.IonicVersion != "" {
		if err := installDependency(packageManager, "ionic", configs.IonicVersion); err != nil {
			fail("%s", err)
		}
	}

	// Print cordova and ionic version
	cordovaVer, err := ionic.CordovaVersion()
	if err != nil {
		fail("Failed to get cordova version, error: %s", err)
	}

	fmt.Println()
	log.Printf("cordova version: %s", colorstring.Green(cordovaVer.String()))

	ionicVer, err := ionic.Version()
	if err != nil {
		fail("Failed to get ionic version, error: %s", err)
	}

	log.Printf("ionic version: %s", colorstring.Green(ionicVer.String()))

	// Ionic CLI plugins angular and cordova have been marked as deprecated for
	// version 3.8.0 and above.
	ionicVerConstraint, err := ver.NewConstraint("< 3.8.0")
	if err != nil {
		fail("Could not create version constraint for ionic: %s", err)
	}
	if ionicVerConstraint.Check(ionicVer) {
		fmt.Println()
		log.Infof("Installing cordova and angular plugins")
		cmd, err := jsdependency.AddCommand(packageManager, jsdependency.Local, "@ionic/cli-plugin-ionic-angular@latest", "@ionic/cli-plugin-cordova@latest")
		if err != nil {
			fail("%s", err)
		}
		fmt.Println()
		log.Donef("$ %s", cmd.PrintableCommandArgs())
		fmt.Println()

		if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
			if errorutil.IsExitStatusError(err) {
				fail("Failed to install: %s failed, output: %s", cmd.PrintableCommandArgs(), out)
			}
			fail("Failed to install: %s failed, error: %s", cmd.PrintableCommandArgs(), err)
		}
	}

	// ionic login
	if configs.Username != "" && configs.Password != "" {
		fmt.Println()
		log.Infof("Ionic login")

		cmd := ionic.LoginCommand(configs.Username, configs.Password)
		cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

		log.Donef("$ ionic login *** ***")

		if err := cmd.Run(); err != nil {
			fail("ionic login command failed, error: %s", err)
		}
	}

	ionicMajorVersion := ionicVer.Segments()[0]

	platforms := strings.Split(configs.Platform, ",")
	for i, p := range platforms {
		platforms[i] = strings.TrimSpace(p)
	}
	sort.Strings(platforms)

	// ionic prepare
	fmt.Println()
	log.Infof("Building project")

	if configs.RunPrepare {
		cmd := ionic.PrepareCommand(ionicMajorVersion)
		cmd.SetStdout(os.Stdout).SetStderr(os.Stderr)

		log.Donef("$ %s", cmd.PrintableCommandArgs())

		if err := cmd.Run(); err != nil {
			fail("ionic prepare command %s failed, error: %s", cmd.PrintableCommandArgs(), err)
		}
	}

	buildStart := time.Now()
	{
		// build
		options := []string{}
		if configs.Options != "" {
			opts, err := shellquote.Split(configs.Options)
			if err != nil {
				fail("Failed to shell split Options (%s), error: %s", configs.Options, err)
			}
			options = opts
		}

		for _, platform := range platforms {
			cmdArgs := []string{"ionic"}
			if ionicMajorVersion > 2 {
				cmdArgs = append(cmdArgs, "cordova")
			}

			cmdArgs = append(cmdArgs, "build")

			if configs.Configuration != "" {
				cmdArgs = append(cmdArgs, "--"+configs.Configuration)
			}

			if configs.Target != "" {
				cmdArgs = append(cmdArgs, "--"+configs.Target)
			}

			cmdArgs = append(cmdArgs, platform)

			if configs.BuildConfig != "" {
				cmdArgs = append(cmdArgs, "--buildConfig", configs.BuildConfig)
			}

			cmdArgs = append(cmdArgs, options...)
			cmdArgs = append(cmdArgs)

			cmd := command.New(cmdArgs[0], cmdArgs[1:]...)
			cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

			log.Donef("$ %s", cmd.PrintableCommandArgs())

			if err := cmd.Run(); err != nil {
				fail("command failed, error: %s", err)
			}
		}
	}

	// collect outputs

	var ipas, dsyms, apps []string
	iosOutputDir := filepath.Join(workDir, "platforms", "ios", "build", configs.Target)
	log.Debugf("iOS output directory: %s", iosOutputDir)
	if exist, err := pathutil.IsDirExists(iosOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", iosOutputDir, err)
	} else if exist {
		log.Donef("\n\nIOS output dir exists!\n\n")

		fmt.Println()
		log.Infof("Collecting ios outputs")

		// ipa
		ipas, err = findArtifact(iosOutputDir, "ipa", buildStart)
		if err != nil {
			fail("Failed to find ipas in dir (%s), error: %s", iosOutputDir, err)
		}

		if len(ipas) > 0 {
			if exportedPth, _, err := moveAndExportOutputs(ipas, configs.DeployDir, ipaPathEnvKey, apkPathListEnvKey); err != nil {
				fail("Failed to export ipas, error: %s", err)
			} else if exportedPth != "" {
				log.Donef("The ipa path is now available in the Environment Variable: %s (value: %s)", ipaPathEnvKey, exportedPth)
			}
		}
		// ---

		// dsym
		dsyms, err = findArtifact(iosOutputDir, "dSYM", buildStart)
		if err != nil {
			fail("Failed to find dSYMs in dir (%s), error: %s", iosOutputDir, err)
		}

		if len(dsyms) > 0 {
			if exportedPth, _, err := moveAndExportOutputs(dsyms, configs.DeployDir, dsymDirPathEnvKey, apkPathListEnvKey); err != nil {
				fail("Failed to export dsyms, error: %s", err)
			} else if exportedPth != "" {
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
		// --

		// app
		apps, err = findArtifact(iosOutputDir, "app", buildStart)
		if err != nil {
			fail("Failed to find apps in dir (%s), error: %s", iosOutputDir, err)
		}

		if len(apps) > 0 {
			if exportedPth, _, err := moveAndExportOutputs(apps, configs.DeployDir, appDirPathEnvKey, apkPathListEnvKey); err != nil {
				log.Warnf("Failed to export apps, error: %s", err)
			} else if exportedPth != "" {
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
		// ---
	} else {
		// ios output directory not exists and ios selected as platform
	}

	var apks []string
	androidOutputDir := filepath.Join(workDir, "platforms", "android")
	log.Debugf("Android output directory: %s", androidOutputDir)
	if exist, err := pathutil.IsDirExists(androidOutputDir); err != nil {
		fail("Failed to check if dir (%s) exist, error: %s", androidOutputDir, err)
	} else if exist {
		fmt.Println()
		log.Infof("Collecting android outputs")

		apks, err = findArtifact(androidOutputDir, "apk", buildStart)
		if err != nil {
			fail("Failed to find apks in dir (%s), error: %s", androidOutputDir, err)
		}

		if len(apks) > 0 {
			if exportedPth, exportedPaths, err := moveAndExportOutputs(apks, configs.DeployDir, apkPathEnvKey, apkPathListEnvKey); err != nil {
				fail("Failed to export apks, error: %s", err)
			} else if exportedPth != "" {
				log.Donef("The apk path is now available in the Environment Variable: %s (value: %s)", apkPathEnvKey, exportedPth)
				if len(exportedPaths) > 0 {
					log.Donef("The apk paths are now available in the Environment Variable: %s (value: %s)", apkPathListEnvKey, strings.Join(exportedPaths, "|"))
				}
			}
		}
	}

	// if android in platforms
	if len(apks) == 0 && sliceutil.IsStringInSlice("android", platforms) {
		fail("No apk generated")
	}
	// if ios in platforms
	if sliceutil.IsStringInSlice("ios", platforms) {
		if len(apps) == 0 && configs.Target == "emulator" {
			fail("no apps generated")
		}
		if len(ipas) == 0 && configs.Target == "device" {
			fail("no ipas generated")
		}
	}
}
