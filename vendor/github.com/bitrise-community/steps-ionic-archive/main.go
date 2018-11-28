package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bitrise-community/steps-ionic-archive/jsdependency"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/tools"
	ver "github.com/hashicorp/go-version"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
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

// ConfigsModel ...
type ConfigsModel struct {
	Platform      string
	Configuration string
	Target        string
	BuildConfig   string
	Options       string

	Username string
	Password string

	ReAddPlatform         string
	IonicVersion          string
	CordovaVersion        string
	CordovaIosVersion     string
	CordovaAndroidVersion string

	WorkDir   string
	DeployDir string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		Platform:      os.Getenv("platform"),
		Configuration: os.Getenv("configuration"),
		Target:        os.Getenv("target"),
		BuildConfig:   os.Getenv("build_config"),
		Options:       os.Getenv("options"),

		Username: os.Getenv("ionic_username"),
		Password: os.Getenv("ionic_password"),

		ReAddPlatform:         os.Getenv("readd_platform"),
		IonicVersion:          os.Getenv("ionic_version"),
		CordovaVersion:        os.Getenv("cordova_version"),
		CordovaIosVersion:     os.Getenv("cordova_ios_version"),
		CordovaAndroidVersion: os.Getenv("cordova_android_version"),

		WorkDir:   os.Getenv("workdir"),
		DeployDir: os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- Platform: %s", configs.Platform)
	log.Printf("- Configuration: %s", configs.Configuration)
	log.Printf("- Target: %s", configs.Target)
	log.Printf("- BuildConfig: %s", configs.BuildConfig)
	log.Printf("- Options: %s", configs.Options)

	log.Printf("- Username: %s", input.SecureInput(configs.Username))
	log.Printf("- Password: %s", input.SecureInput(configs.Password))

	log.Printf("- ReAddPlatform: %s", configs.ReAddPlatform)
	log.Printf("- IonicVersion: %s", configs.IonicVersion)
	log.Printf("- CordovaVersion: %s", configs.CordovaVersion)
	log.Printf("- CordovaIosVersion: %s", configs.CordovaIosVersion)
	log.Printf("- CordovaAndroidVersion: %s", configs.CordovaAndroidVersion)

	log.Printf("- WorkDir: %s", configs.WorkDir)
	log.Printf("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if err := input.ValidateIfDirExists(configs.WorkDir); err != nil {
		return fmt.Errorf("WorkDir: %s", err)
	}

	if err := input.ValidateWithOptions(configs.Platform, "ios,android", "ios", "android"); err != nil {
		return fmt.Errorf("Platform: %s", err)
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

func ionicVersion() (*ver.Version, error) {
	cmd := command.New("ionic", "-v")
	cmd.SetStdin(strings.NewReader("Y"))
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}

	// fix for ionic-cli intercative version output: `[1000D[K3.2.0`
	pattern := `(?P<version>\d+\.\d+\.\d+)`

	reader := strings.NewReader(out)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if match := regexp.MustCompile(pattern).FindStringSubmatch(line); len(match) == 2 {
			versionStr := match[1]
			version, err := ver.NewVersion(versionStr)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse version: %s", versionStr)
			}
			return version, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("output: %s", out)
}

func cordovaVersion() (*ver.Version, error) {
	cmd := command.New("cordova", "-v")
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}
	out = strings.Split(out, "(")[0]
	out = strings.TrimSpace(out)

	version, err := ver.NewVersion(out)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse version: %s", out)
	}
	return version, nil
}

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func getField(c ConfigsModel, field string) string {
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	return string(f.String())
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
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		fail("Issue with input: %s", err)
	}

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
	packageManager := jsdependency.DetectTool(workDir)
	log.Printf("Js package manager used: %s", packageManager)
	if configs.CordovaVersion != "" {
		fmt.Println()
		log.Infof("Updating cordova version to: %s", configs.CordovaVersion)

		// Yarn returns an error if the package is not added before removal, ignoring
		if err := jsdependency.Remove(packageManager, jsdependency.Local, "cordova"); err != nil && packageManager != jsdependency.Yarn {
			fail("Failed to remove local cordova, error: %s", err)
		}

		if err := jsdependency.Add(packageManager, jsdependency.Global, "cordova@"+configs.CordovaVersion); err != nil {
			fail("Failed to install cordova, error: %s", err)
		}
	}

	if configs.IonicVersion != "" {
		fmt.Println()
		log.Infof("Updating ionic version to: %s", configs.IonicVersion)

		if err := jsdependency.Remove(packageManager, jsdependency.Local, "cordova"); err != nil && packageManager != jsdependency.Yarn {
			fail("Failed to remove local ionic, error: %s", err)
		}

		if err := jsdependency.Add(packageManager, jsdependency.Global, "ionic@"+configs.IonicVersion); err != nil {
			fail("Failed to install ionic, error: %s", err)
		}
	}

	// Print cordova and ionic version
	cordovaVer, err := cordovaVersion()
	if err != nil {
		fail("Failed to get cordova version, error: %s", err)
	}

	fmt.Println()
	log.Printf("cordova version: %s", colorstring.Green(cordovaVer.String()))

	ionicVer, err := ionicVersion()
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
		if err := jsdependency.Add(packageManager, jsdependency.Local, "@ionic/cli-plugin-ionic-angular@latest", "@ionic/cli-plugin-cordova@latest"); err != nil {
			fail("command failed, error: %s", err)
		}
	}

	//
	// ionic login
	if configs.Username != "" && configs.Password != "" {
		fmt.Println()
		log.Infof("Ionic login")

		cmdArgs := []string{"ionic", "login", configs.Username, configs.Password}
		cmd := command.New(cmdArgs[0], cmdArgs[1:]...)
		cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

		log.Donef("$ ionic login *** ***")

		if err := cmd.Run(); err != nil {
			fail("command failed, error: %s", err)
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

	// platform rm
	if configs.ReAddPlatform == "true" {
		for _, platform := range platforms {
			cmdArgs := []string{"ionic"}
			if ionicMajorVersion > 2 {
				cmdArgs = append(cmdArgs, "cordova")
			}

			cmdArgs = append(cmdArgs, "platform", "rm")

			cmdArgs = append(cmdArgs, platform)

			cmd := command.New(cmdArgs[0], cmdArgs[1:]...)
			cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

			log.Donef("$ %s", cmd.PrintableCommandArgs())

			if err := cmd.Run(); err != nil {
				fail("command failed, error: %s", err)
			}
		}
	}

	{
		// platform add
		for _, platform := range platforms {
			cmdArgs := []string{"ionic"}
			if ionicMajorVersion > 2 {
				cmdArgs = append(cmdArgs, "cordova")
			}

			cmdArgs = append(cmdArgs, "platform", "add")

			platformVersion := platform
			pv := getField(configs, "Cordova"+strings.Title(platform)+"Version")
			if pv == "master" {
				platformVersion = "https://github.com/apache/cordova-" + platform + ".git"
			} else if pv != "" {
				platformVersion = platform + "@" + pv
			}

			cmdArgs = append(cmdArgs, platformVersion)

			cmd := command.New(cmdArgs[0], cmdArgs[1:]...)
			cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

			log.Donef("$ %s", cmd.PrintableCommandArgs())

			if err := cmd.Run(); err != nil {
				fail("command failed, error: %s", err)
			}
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
