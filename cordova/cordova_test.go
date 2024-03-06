package cordova

import (
	"testing"
)

func Test_platformCustomOptions(t *testing.T) {
	tests := []struct {
		name    string
		model   Model
		wantCmd string
	}{
		{
			"Project with generic option and AAB output",
			*New().SetCustomOptions("--release").SetPlatforms("android").SetAndroidAppType("aab"),
			`cordova "compile" "android" "--release" "--" "--packageType=bundle"`,
		},
		{
			"Project with generic option and APK output",
			*New().SetCustomOptions("--release").SetPlatforms("android").SetAndroidAppType("apk"),
			`cordova "compile" "android" "--release" "--" "--packageType=apk"`,
		},
		{
			"Project with no generic option and APK output",
			*New().SetPlatforms("android").SetAndroidAppType("apk"),
			`cordova "compile" "android" "--" "--packageType=apk"`,
		},
		{
			"Project with no generic option and AAB output",
			*New().SetPlatforms("android").SetAndroidAppType("aab"),
			`cordova "compile" "android" "--" "--packageType=bundle"`,
		},
		{
			"Project with platform-specific and generic options and AAB output",
			*New().SetCustomOptions("--release", "--", "--keystore=android.keystore").SetPlatforms("android").SetAndroidAppType("aab"),
			`cordova "compile" "android" "--release" "--" "--packageType=bundle" "--keystore=android.keystore"`,
		},
		{
			"Project with platform-specific and generic options and APK output",
			*New().SetCustomOptions("--release", "--", "--keystore=android.keystore").SetPlatforms("android").SetAndroidAppType("apk"),
			`cordova "compile" "android" "--release" "--" "--packageType=apk" "--keystore=android.keystore"`,
		},
		{
			"Project with only packageType option and APK output",
			*New().SetCustomOptions("--", `--packageType="bundle"`).SetPlatforms("android").SetAndroidAppType("apk"),
			`cordova "compile" "android" "--" "--packageType=apk" "--packageType="bundle""`,
		},
		{
			"Project with only packageType option and AAB output",
			*New().SetCustomOptions("--", `--packageType="bundle"`).SetPlatforms("android").SetAndroidAppType("bundle"),
			`cordova "compile" "android" "--" "--packageType=bundle" "--packageType="bundle""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.model.CompileCommand().PrintableCommandArgs()
			if tt.wantCmd != cmd {
				t.Errorf("Cordova command doesn't match wanted value.\nWant: %v\nGot: %s", tt.wantCmd, cmd)
			}
		})
	}
}
