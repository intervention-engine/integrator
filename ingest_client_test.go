package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"os"

	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestIngestClientSuite(t *testing.T) {
	suite.Run(t, new(IngestClientSuite))
}

type IngestClientSuite struct {
	suite.Suite
	Client              *HttpIngestClient
	Server              *httptest.Server
	ReceivedContentType string
	ReceivedContent     string
	Respond500          bool
}

func (suite *IngestClientSuite) SetupTest() {
	suite.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.ReceivedContentType = r.Header.Get("Content-Type")
		defer r.Body.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		suite.ReceivedContent = buf.String()
		if suite.Respond500 {
			w.WriteHeader(500)
		}
	}))

	suite.Client = NewHttpIngestClient(suite.Server.URL)
}

func (suite *IngestClientSuite) TearDownTest() {
	if suite.Server != nil {
		suite.Server.Close()
	}
	suite.ReceivedContentType = ""
	suite.ReceivedContent = ""
	suite.Respond500 = false
}

func (suite *IngestClientSuite) TestSuccessfulIngest() {
	assert := suite.Assert()
	require := suite.Require()

	f, err := os.Open("./fixtures/document.xml")
	require.NoError(err)
	err = suite.Client.Ingest("text/xml", f)
	require.NoError(err)
	assert.Equal("text/xml", suite.ReceivedContentType)
	assert.Equal("<document>\n    <foo>bar</foo>\n</document>", suite.ReceivedContent)
}

func (suite *IngestClientSuite) TestErrorIngest() {
	require := suite.Require()

	suite.Respond500 = true
	f, err := os.Open("./fixtures/document.xml")
	require.NoError(err)
	err = suite.Client.Ingest("text/xml", f)
	require.Error(err)
}
