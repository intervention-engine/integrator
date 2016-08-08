package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHIEClientSuite(t *testing.T) {
	suite.Run(t, new(HIEClientSuite))
}

type HIEClientSuite struct {
	suite.Suite
	Client      *HttpHieClient
	Server      *httptest.Server
	LastRequest *url.URL
	Respond400  bool
}

func (suite *HIEClientSuite) SetupTest() {
	require := suite.Require()

	querySuccess, err := os.Open("./fixtures/response_success.json")
	require.NoError(err)
	queryFailure, err := os.Open("./fixtures/response_error.json")
	require.NoError(err)
	documentSuccess, err := os.Open("./fixtures/document.xml")
	require.NoError(err)
	documentFailure, err := os.Open("./fixtures/document_error.json")
	require.NoError(err)
	suite.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.LastRequest = r.URL
		if strings.Contains(r.URL.Path, "/docs/") {
			if suite.Respond400 {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(400)
				io.Copy(w, documentFailure)
			} else {
				w.Header().Set("Content-Type", "text/xml; charset=utf-8")
				io.Copy(w, documentSuccess)
			}
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if suite.Respond400 {
				w.WriteHeader(400)
				io.Copy(w, queryFailure)
			} else {
				io.Copy(w, querySuccess)
			}
		}
	}))

	suite.Client = NewHttpHieClient(suite.Server.URL)
}

func (suite *HIEClientSuite) TearDownTest() {
	if suite.Server != nil {
		suite.Server.Close()
	}
	suite.LastRequest = nil
	suite.Respond400 = false
}

func (suite *HIEClientSuite) TestUnmarshalSuccessfulResponse() {
	assert := suite.Assert()
	require := suite.Require()

	b, err := ioutil.ReadFile("./fixtures/response_success.json")
	require.NoError(err)

	var r QueryResponse
	json.Unmarshal(b, &r)
	assert.True(r.Status)
	assert.Len(r.Result, 3)
	assert.Equal(QueryResponseEntry{
		RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.1",
		CreationTime: time.Date(2014, 4, 25, 2, 51, 3, 0, time.Local),
		Title:        "Test Continuity of Care",
		DocumentType: "XML^HL7^231^CCD^C32",
		DocumentID:   "1.1.1.1.1.1",
		Hash:         "4C167E7B7F006A18ABB2E4A1A9B2489936947E91",
		Size:         28452,
	}, r.Result[0])
	assert.Equal(QueryResponseEntry{
		RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.2",
		CreationTime: time.Date(2014, 4, 25, 2, 14, 3, 0, time.Local),
		Title:        "Test Continuity of Care",
		DocumentType: "XML^HL7^231^CCD^C32",
		DocumentID:   "1.1.1.1.1.2",
		Hash:         "5B885732FE2D9D33AAEBBDA3CCE01A2F1D279E13",
		Size:         27869,
	}, r.Result[1])
	assert.Equal(QueryResponseEntry{
		RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.3",
		CreationTime: time.Date(2013, 12, 9, 5, 7, 3, 0, time.Local),
		Title:        "Test Clinical Summary",
		DocumentType: "XML^HL7^231^CCD^C32",
		DocumentID:   "1.1.1.1.1.3",
		Hash:         "B6983379C28B50FF5D2A383BF0F0B6B6FDAFDD1B",
		Size:         17028,
	}, r.Result[2])
	assert.Equal(QueryRequest{
		Env:                   "test",
		Host:                  "test.foo.net",
		EE:                    "123456789",
		StartDateTime:         time.Date(2010, 1, 1, 0, 0, 0, 0, time.Local),
		EndDateTime:           time.Date(2016, 6, 8, 23, 59, 59, 0, time.Local),
		QueryStartDateTime:    time.Date(2016, 6, 8, 21, 12, 35, 17053400, time.UTC),
		QueryCompleteDateTime: time.Date(2016, 6, 8, 21, 13, 2, 687820200, time.UTC),
	}, r.Query)
}

func (suite *HIEClientSuite) TestUnmarshalSuccessfulError() {
	assert := suite.Assert()
	require := suite.Require()

	b, err := ioutil.ReadFile("./fixtures/response_error.json")
	require.NoError(err)

	var r QueryResponse
	json.Unmarshal(b, &r)
	assert.False(r.Status)
	assert.Equal("invalid ee", r.Error)
	assert.Len(r.Result, 0)
	assert.Equal(QueryRequest{
		Env:                   "test",
		Host:                  "test.foo.net",
		EE:                    "-987654321",
		StartDateTime:         time.Date(2010, 1, 1, 0, 0, 0, 0, time.Local),
		EndDateTime:           time.Date(2016, 6, 8, 23, 59, 59, 0, time.Local),
		QueryStartDateTime:    time.Date(2016, 6, 8, 21, 12, 35, 17053400, time.UTC),
		QueryCompleteDateTime: time.Date(2016, 6, 8, 21, 13, 2, 687820200, time.UTC),
	}, r.Query)
}

func (suite *HIEClientSuite) TestQueryRecordsNoDates() {
	assert := suite.Assert()
	require := suite.Require()

	resp, err := suite.Client.QueryRecords("123", nil, nil)
	require.NoError(err)
	req := suite.LastRequest
	params := req.Query()
	assert.Len(params, 1)
	assert.Equal("123", params.Get("ee"))

	// We test deserialization details elsewhere, so just do a sniff test here
	require.NotNil(resp)
	assert.True(resp.Status)
	assert.Empty(resp.Error)
	assert.Equal("test", resp.Query.Env)
	assert.Len(resp.Result, 3)
}

func (suite *HIEClientSuite) TestQueryRecordsWithStartDate() {
	assert := suite.Assert()
	require := suite.Require()

	start := time.Date(2016, time.May, 1, 10, 20, 30, 0, time.Local)
	resp, err := suite.Client.QueryRecords("123", &start, nil)
	require.NoError(err)
	req := suite.LastRequest
	params := req.Query()
	assert.Len(params, 2)
	assert.Equal("123", params.Get("ee"))
	assert.Equal("2016-05-01T10:20:30", params.Get("startDateTime"))

	// We test deserialization details elsewhere, so just do a sniff test here
	require.NotNil(resp)
	assert.True(resp.Status)
	assert.Empty(resp.Error)
	assert.Equal("test", resp.Query.Env)
	assert.Len(resp.Result, 3)
}

func (suite *HIEClientSuite) TestQueryRecordsWithEndDate() {
	assert := suite.Assert()
	require := suite.Require()

	end := time.Date(2016, time.June, 1, 10, 20, 30, 0, time.Local)
	resp, err := suite.Client.QueryRecords("123", nil, &end)
	require.NoError(err)
	req := suite.LastRequest
	params := req.Query()
	assert.Len(params, 2)
	assert.Equal("123", params.Get("ee"))
	assert.Equal("2016-06-01T10:20:30", params.Get("endDateTime"))

	// We test deserialization details elsewhere, so just do a sniff test here
	require.NotNil(resp)
	assert.True(resp.Status)
	assert.Empty(resp.Error)
	assert.Equal("test", resp.Query.Env)
	assert.Len(resp.Result, 3)
}

func (suite *HIEClientSuite) TestQueryRecordsWithStartAndEndDate() {
	assert := suite.Assert()
	require := suite.Require()

	start := time.Date(2016, time.May, 1, 10, 20, 30, 0, time.Local)
	end := time.Date(2016, time.June, 1, 10, 20, 30, 0, time.Local)
	resp, err := suite.Client.QueryRecords("123", &start, &end)
	require.NoError(err)
	req := suite.LastRequest
	params := req.Query()
	assert.Len(params, 3)
	assert.Equal("123", params.Get("ee"))
	assert.Equal("2016-05-01T10:20:30", params.Get("startDateTime"))
	assert.Equal("2016-06-01T10:20:30", params.Get("endDateTime"))

	// We test deserialization details elsewhere, so just do a sniff test here
	require.NotNil(resp)
	assert.True(resp.Status)
	assert.Empty(resp.Error)
	assert.Equal("test", resp.Query.Env)
	assert.Len(resp.Result, 3)
}

func (suite *HIEClientSuite) TestQueryRecordsError() {
	assert := suite.Assert()
	require := suite.Require()

	suite.Respond400 = true
	resp, err := suite.Client.QueryRecords("-123", nil, nil)
	require.NotNil(err)
	assert.Contains(err.Error(), "400")
	assert.Contains(err.Error(), "Bad Request")

	require.NotNil(resp)
	assert.False(resp.Status)
	assert.Equal("invalid ee", resp.Error)
	assert.Equal("test", resp.Query.Env)
	assert.Empty(resp.Result)
}

func (suite *HIEClientSuite) TestDownloadRecord() {
	assert := suite.Assert()
	require := suite.Require()

	content, cType, err := suite.Client.DownloadRecord(suite.Server.URL + "/docs/123")
	require.NoError(err)

	require.NotNil(content)
	defer content.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(content)
	assert.Equal("<document>\n    <foo>bar</foo>\n</document>", buf.String())

	assert.Equal("text/xml; charset=utf-8", cType)
}

func (suite *HIEClientSuite) TestDownloadRecordError() {
	assert := suite.Assert()
	require := suite.Require()

	suite.Respond400 = true
	content, cType, err := suite.Client.DownloadRecord(suite.Server.URL + "/docs/-123")

	require.NotNil(err)
	assert.Equal("invalid document ID", err.Error())
	assert.Nil(content)
	assert.Empty(cType)
}

func (suite *HIEClientSuite) TestLenientParse() {
	assert := suite.Assert()
	require := suite.Require()

	t, err := lenientParse("2006-01-02T15:04:05.000000000Z", "2016-06-08T21:13:02.123456789Z")
	require.Nil(err)
	assert.Equal(time.Date(2016, 6, 8, 21, 13, 2, 123456789, time.UTC), t)

	t, err = lenientParse("2006-01-02T15:04:05.000000000Z", "2016-06-08T21:13:02.12345678Z")
	require.Nil(err)
	assert.Equal(time.Date(2016, 6, 8, 21, 13, 2, 123456780, time.UTC), t)
	t, err = lenientParse("2006-01-02T15:04:05.000000000Z", "2016-06-08T21:13:02.1234567Z")
	require.Nil(err)
	assert.Equal(time.Date(2016, 6, 8, 21, 13, 2, 123456700, time.UTC), t)
	t, err = lenientParse("2006-01-02T15:04:05.000000000Z", "2016-06-08T21:13:02.123456Z")
	require.Nil(err)
	assert.Equal(time.Date(2016, 6, 8, 21, 13, 2, 123456000, time.UTC), t)
	t, err = lenientParse("2006-01-02T15:04:05.000000000Z", "2016-06-08T21:13:02.12345Z")
	require.Nil(err)
	assert.Equal(time.Date(2016, 6, 8, 21, 13, 2, 123450000, time.UTC), t)
}
