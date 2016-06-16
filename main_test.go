package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestMainSuite(t *testing.T) {
	suite.Run(t, new(MainSuite))
}

type MainSuite struct {
	suite.Suite
}

func (suite *MainSuite) TestParseEEFile() {
	assert := suite.Assert()
	require := suite.Require()

	ees, err := parseEEFile("./fixtures/ee_file.txt")
	require.NoError(err)
	assert.Len(ees, 8)
	assert.Equal([]string{
		"123",
		"ABC",
		"456",
		"XYZ",
		"789",
		"SPACESBEFORE",
		"SPACESAFTER",
		"THE END",
	}, ees)
}
