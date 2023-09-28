package main

import "testing"

func Test_checkBuildProducts(t *testing.T) {
	type args struct {
		apks      []string
		aabs      []string
		apps      []string
		ipas      []string
		platforms []string
		target    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"No build products FAIL",
			args{
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{"ios", "android"},
				"emulator",
			},
			true,
		},
		{
			"No android FAIL, ios generated",
			args{
				[]string{},
				[]string{},
				[]string{"/path.app"},
				[]string{},
				[]string{"ios", "android"},
				"emulator",
			},
			true,
		},
		{
			"No ios FAIL, android apk generated",
			args{
				[]string{"/path.apk"},
				[]string{},
				[]string{},
				[]string{},
				[]string{"ios", "android"},
				"emulator",
			},
			true,
		},
		{
			"No ios FAIL, android aab generated",
			args{
				[]string{},
				[]string{"/path.aab"},
				[]string{},
				[]string{},
				[]string{"ios", "android"},
				"device",
			},
			true,
		},
		{
			"ios emulator target, app generated, OK",
			args{
				[]string{},
				[]string{},
				[]string{"/path.app"},
				[]string{},
				[]string{"ios"},
				"emulator",
			},
			false,
		},
		{
			"ios emulator target, ipa generated, FAIL",
			args{
				[]string{},
				[]string{},
				[]string{},
				[]string{"/path.ipa"},
				[]string{"ios"},
				"emulator",
			},
			true,
		},
		{
			"ios device target, ipa generated, OK",
			args{
				[]string{},
				[]string{},
				[]string{},
				[]string{"/app_path.ipa"},
				[]string{"ios"},
				"device",
			},
			false,
		},
		{
			"ios device target, app generated, FAIL",
			args{
				[]string{},
				[]string{},
				[]string{"/app_path.app"},
				[]string{},
				[]string{"ios"},
				"device",
			},
			true,
		},
		{
			"Android aab only, OK",
			args{
				[]string{"/path.apk"},
				[]string{},
				[]string{},
				[]string{"/path.ipa"},
				[]string{"ios", "android"},
				"device",
			},
			false,
		},
		{
			"ios, android OK",
			args{
				[]string{"/path.apk"},
				[]string{},
				[]string{},
				[]string{"/path.ipa"},
				[]string{"ios", "android"},
				"device",
			},
			false,
		},
		{
			"ios, android OK",
			args{
				[]string{},
				[]string{"/path.aab"},
				[]string{},
				[]string{"/path.ipa"},
				[]string{"ios", "android"},
				"device",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkBuildProducts(tt.args.apks, tt.args.aabs, tt.args.apps, tt.args.ipas, tt.args.platforms, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("checkBuildProducts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_findIosTargetPathComponentEmulatorOld(t *testing.T) {
    got := findIosTargetPathComponent("emulator", "debug", "6.3.0")
    want := "emulator"
    if got != want {
        t.Errorf("got %q, wanted %q", got, want)
    }
}

func Test_findIosTargetPathComponentDeviceOld(t *testing.T) {
    got := findIosTargetPathComponent("device", "debug", "6.3.0")
    want := "device"
    if got != want {
        t.Errorf("got %q, wanted %q", got, want)
    }
}

func Test_findIosTargetPathComponentEmulatorDebug7(t *testing.T) {
    got := findIosTargetPathComponent("emulator", "debug", "7.0.0")
    want := "Debug-iphonesimulator"
    if got != want {
        t.Errorf("got %q, wanted %q", got, want)
    }
}

func Test_findIosTargetPathComponentEmulatorRelease7(t *testing.T) {
    got := findIosTargetPathComponent("emulator", "release", "7.0.0")
    want := "Release-iphonesimulator"
    if got != want {
        t.Errorf("got %q, wanted %q", got, want)
    }
}

func Test_findIosTargetPathComponentDeviceRelease7(t *testing.T) {
    got := findIosTargetPathComponent("device", "release", "7.0.0")
    want := "Release-iphoneos"
    if got != want {
        t.Errorf("got %q, wanted %q", got, want)
    }
}

func Test_findIosTargetPathComponentDeviceDebug10Plus(t *testing.T) {
    got := findIosTargetPathComponent("device", "debug", "99.0.0")
    want := "Debug-iphoneos"
    if got != want {
        t.Errorf("got %q, wanted %q", got, want)
    }
}
