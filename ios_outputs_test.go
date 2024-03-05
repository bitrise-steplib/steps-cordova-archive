package main

import (
	"testing"
)

func Test_getIosOutputCandidateDirsPaths(t *testing.T) {
	testCases := []struct {
		name          string
		target        string
		configuration string
		want          []string
	}{
		{
			name:          "Device + debug",
			target:        "device",
			configuration: "debug",
			want: []string{
				"/workdir/platforms/ios/build/device",
				"/workdir/platforms/ios/build/Debug-iphoneos",
			},
		},
		{
			name:          "Emulator + release",
			target:        "emulator",
			configuration: "release",
			want: []string{
				"/workdir/platforms/ios/build/emulator",
				"/workdir/platforms/ios/build/Release-iphonesimulator",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := getIosOutputCandidateDirsPaths("/workdir", tc.target, tc.configuration)
			if len(got) != len(tc.want) {
				t.Fatalf("got len %v not equal to want len %v", got, tc.want)
			}
			for i, wantElem := range tc.want {
				if wantElem != got[i] {
					t.Fatalf("got %v not equal to want %v", got, tc.want)
				}
			}
		})
	}
}
