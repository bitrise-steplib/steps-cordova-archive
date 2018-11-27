package main

import "testing"

func Test_checkBuildProducts(t *testing.T) {
	type args struct {
		apks      []string
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
				[]string{"ios", "android"},
				"emulator",
			},
			true,
		},
		{
			"No android FAIL, ios generated",
			args{
				[]string{},
				[]string{"/path.app"},
				[]string{},
				[]string{"ios", "android"},
				"emulator",
			},
			true,
		},
		{
			"No ios FAIL, android generated",
			args{
				[]string{"/path.apk"},
				[]string{},
				[]string{},
				[]string{"ios", "android"},
				"emulator",
			},
			true,
		},
		{
			"ios emulator target OK",
			args{
				[]string{},
				[]string{"/path.app"},
				[]string{},
				[]string{"ios"},
				"emulator",
			},
			false,
		},
		{
			"ios emulator target, ipa generated FAIL",
			args{
				[]string{},
				[]string{},
				[]string{"/path.apk"},
				[]string{"ios"},
				"emulator",
			},
			true,
		},
		{
			"ios device target, app generated FAIL",
			args{
				[]string{},
				[]string{"/app_path"},
				[]string{},
				[]string{"ios"},
				"device",
			},
			true,
		},
		{
			"ios, android OK",
			args{
				[]string{"/path.apk"},
				[]string{},
				[]string{"/path.ipa"},
				[]string{"ios, android"},
				"device",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkBuildProducts(tt.args.apks, tt.args.apps, tt.args.ipas, tt.args.platforms, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("checkBuildProducts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
