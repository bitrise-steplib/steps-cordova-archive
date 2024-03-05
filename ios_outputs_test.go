package main

import (
	"testing"

	"github.com/stretchr/testify/require"
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
			require.Equal(t, tc.want, got)
		})
	}
}
