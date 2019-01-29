package soap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type newWsseAuthInfoTest struct {
	name       string
	inCertPath string
	inKeyPath  string
	err        error
}

var newWsseAuthInfoTests = []newWsseAuthInfoTest{
	{
		name:       "base case",
		inCertPath: "./testdata/cert.pem",
		inKeyPath:  "./testdata/key.pem",
		err:        nil,
	},
	{
		name:       "invalid key file case",
		inCertPath: "./testdata/cert.pem",
		inKeyPath:  "./testdata/badkey.pem",
		err:        ErrInvalidPEMFileSpecified,
	},
}

func TestNewWSSEAuthInfo(t *testing.T) {
	for _, tt := range newWsseAuthInfoTests {
		t.Run(tt.name, func(t *testing.T) {
			wsseInfo, err := NewWSSEAuthInfo(tt.inCertPath, tt.inKeyPath)
			assert.Equal(t, tt.err, err)
			if tt.err == nil {
				assert.NotNil(t, wsseInfo)
			}
		})
	}
}
