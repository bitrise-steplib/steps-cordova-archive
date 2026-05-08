# Cordova Archive

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-cordova-archive?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-cordova-archive/releases)

Creates an archive of your Cordova project by calling `cordova prepare` and then `cordova compile`, using your Cordova build configuration.

<details>
<summary>Description</summary>


The Step creates an archive of your Cordova project: it prepares the project by calling `cordova prepare` and then archives it by calling `cordova compile` with the Cordova CLI.

If you want to perform code signing on your app, the Step requires the **Generate Cordova build configuration** Step: this Step provides the configuration for the **Cordova Archive** Step.

### Configuring the Step

The Step needs to know the platform (iOS, Android, or both), the mode (release or debug), and the target (device or emulator) of your build. You decide whether you want the Step to run the `cordova prepare` command or you want to use the **Cordova Prepare** Step.

1. In the **Platform to use in cordova-cli commands** input, set the platforms you need.
1. In the **Build command configuration** input, set the build mode for the app.

   This can be either `release` or `debug`.

1. In the **Build command target** input, set whether you want to build the app for a device or an emulator.

1. If you use the **Cordova Prepare** Step, set the **Should `cordova prepare` be executed before `cordova compile`?** input to `false`.

1. If you want to deploy your app, the **Build configuration path to describe code signing properties** input should be set to `$BITRISE_CORDOVA_BUILD_CONFIGURATION`.

   This Environment Variable is exposed by the **Generate Cordova build configuration** Step.

### Troubleshooting

- If you run a `release` build, make sure that your code signing configurations are correct. The Step will fail if the **Generate Cordova build configuration** Step does not have the required code signing inputs - for example, if you mean to deploy an iOS app to the App Store, you need a Distribution code signing identity. And of course check the code signing files that you uploaded to Bitrise!

### Useful links

- [Getting started with Ionic/Cordova apps](https://devcenter.bitrise.io/getting-started/getting-started-with-ionic-cordova-apps/)

### Related Steps

- [Generate Cordova build configuration](https://www.bitrise.io/integrations/steps/generate-cordova-build-configuration)
- [Cordova Prepare](https://www.bitrise.io/integrations/steps/cordova-prepare)
- [Manipulate Cordova config.xml](https://www.bitrise.io/integrations/steps/cordova-config)
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/steps/adding-steps-to-a-workflow.html).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `platform` | Specify this input to apply cordova-cli commands to the desired platforms only.  `cordova build [OTHER_PARAMS] <platform>` | required | `ios,android` |
| `configuration` | Specify build command configuration.  `cordova build [OTHER_PARAMS] [--release \| --debug]` | required | `release` |
| `target` | Specify build command target.  `cordova build [OTHER_PARAMS] [--device \| --emulator]` | required | `device` |
| `build_config` | Path to the build configuration file (build.json), which describes code signing properties. |  | `$BITRISE_CORDOVA_BUILD_CONFIGURATION` |
| `run_cordova_prepare` | Should be left at the default (true) value, except if the cordova-prepare step is used.  - true: `cordova prepare <platform>` followed by `cordova compile <platform>` - false: `cordova compile <platform>` | required | `true` |
| `cordova_version` | The version of cordova you want to use.  If the value is set to `latest`, the step will update to the latest cordova version. Leave this input field empty to use the preinstalled cordova version. |  |  |
| `workdir` | Root directory of your Cordova project, where your Cordova config.xml exists. | required | `$BITRISE_SOURCE_DIR` |
| `options` | Use this input to specify custom options, to append to the end of the cordova-cli build command.  The new Xcode build system is now supported in cordova-ios@5.0.0 (https://github.com/apache/cordova-ios/issues/407). Example: - `--browserify`  `cordova build [OTHER_PARAMS] [options]` |  |  |
| `build_system` | The Xcode build system to use.  - legacy: Use the legacy build system. - modern: Use the new Xcode build system. | required | `modern` |
| `cache_local_deps` | Select if the contents of node_modules directory should be cached. `true`: Mark local dependencies to be cached. `false`: Do not use cache.  | required | `false` |
| `android_app_type` | Distribution type when building the Android app | required | `apk` |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_IPA_PATH` | The created iOS .ipa file's path. |
| `BITRISE_APP_DIR_PATH` | The created iOS .app directory's path. |
| `BITRISE_APP_PATH` | The created iOS .app.zip file's path. |
| `BITRISE_DSYM_DIR_PATH` | The created iOS .dSYM directory's path. |
| `BITRISE_DSYM_PATH` | The created iOS .dSYM.zip file's path. |
| `BITRISE_APK_PATH` | The created Android .apk file's path. |
| `BITRISE_AAB_PATH` | The created Android .aab file's path. |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-cordova-archive/pulls) and [issues](https://github.com/bitrise-steplib/steps-cordova-archive/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://docs.bitrise.io/en/bitrise-ci/bitrise-cli/running-your-first-local-build-with-the-cli.html).

Learn more about developing steps:

- [Create your own step](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/developing-your-own-bitrise-step/developing-a-new-step.html)
