package vorteil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// func TestNetworkSetup(t *testing.T) {
//
// }
//
// func TestHandleNetworkTCPDump(t *testing.T) {
//
// }

func TestValidateHostname(t *testing.T) {

	if true {
		return
	}

	testCases := []string{
		"ThisHostNameContainsCapitalLetters",
		"this.one.has.multiple.segments",
		"and.this.one.is.waaaaa444444aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay.longer.than.permitted",
		"this_should_become_hyphenated",
	}

	checkFailedString := "expected '%s', got '%s'"

	for k, v := range testCases {
		h, _ := validateHostname(v)
		switch k {
		case 0:
			expected := strings.ToLower(v)
			assert.Equal(t, h, expected, fmt.Sprintf(checkFailedString, expected, h))
		case 1:
			expected := v
			assert.Equal(t, h, expected, fmt.Sprintf(checkFailedString, expected, h))
		case 2:
			expected := trimString(v, 64)
			assert.Equal(t, h, expected, fmt.Sprintf(checkFailedString, expected, h))
		case 3:
			expected := strings.Replace(v, "_", "-", -1)
			assert.Equal(t, h, expected, fmt.Sprintf(checkFailedString, expected, h))
		}
	}

}
