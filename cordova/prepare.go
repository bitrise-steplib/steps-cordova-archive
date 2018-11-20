package cordova

import (
	"fmt"
	"strings"

	"github.com/bitrise-io/go-utils/command"
)

// CurrentVersion returns current cordova version
func CurrentVersion() (string, error) {
	return toolVersion("cordova")
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
