package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDataCopierSuite(t *testing.T) {
	suite.Run(t, new(DataCopierSuite))
}

type DataCopierSuite struct {
	suite.Suite
	hieClient    *MockHieClient
	ingestClient *MockIngestClient
	txLogMgr     *MockTransactionLogManager
	dataCopier   *DataCopier
}

type MockHieClient struct {
	QueryRecordsFnIndex   int
	QueryRecordsFns       []func(string, *time.Time, *time.Time) (*QueryResponse, error)
	DownloadRecordFnIndex int
	DownloadRecordFns     []func(string) (io.ReadCloser, string, error)
}

func (m *MockHieClient) QueryRecords(mrn string, start *time.Time, end *time.Time) (*QueryResponse, error) {
	i := m.QueryRecordsFnIndex
	m.QueryRecordsFnIndex++
	return m.QueryRecordsFns[i](mrn, start, end)
}

func (m *MockHieClient) DownloadRecord(url string) (content io.ReadCloser, contentType string, err error) {
	i := m.DownloadRecordFnIndex
	m.DownloadRecordFnIndex++
	return m.DownloadRecordFns[i](url)
}

type MockIngestClient struct {
	IngestFnIndex int
	IngestFns     []func(string, io.ReadCloser) error
}

func (m *MockIngestClient) Ingest(contentType string, reader io.ReadCloser) error {
	i := m.IngestFnIndex
	m.IngestFnIndex++
	return m.IngestFns[i](contentType, reader)
}

type MockTransactionLogManager struct {
	FindEntriesFnIndex int
	FindEntriesFns     []func(string) ([]*TransactionLogEntry, error)
	StoreEntryFnIndex  int
	StoreEntryFns      []func(*TransactionLogEntry) error
}

func (m *MockTransactionLogManager) FindEntriesByEE(ee string) (entries []*TransactionLogEntry, err error) {
	i := m.FindEntriesFnIndex
	m.FindEntriesFnIndex++
	return m.FindEntriesFns[i](ee)
}

func (m *MockTransactionLogManager) StoreEntry(entry *TransactionLogEntry) error {
	i := m.StoreEntryFnIndex
	m.StoreEntryFnIndex++
	return m.StoreEntryFns[i](entry)
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func (suite *DataCopierSuite) SetupTest() {
	require := suite.Require()

	var err error
	suite.hieClient = &MockHieClient{}
	suite.ingestClient = &MockIngestClient{}
	suite.txLogMgr = &MockTransactionLogManager{}
	suite.dataCopier, err = NewDataCopier(suite.hieClient, suite.ingestClient, suite.txLogMgr)
	require.NoError(err)
}

func (suite *DataCopierSuite) TestSuccessfulOperation() {
	assert := suite.Assert()
	require := suite.Require()

	// Setup all the mocks
	qStart := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.Local)
	suite.hieClient.QueryRecordsFns = append(suite.hieClient.QueryRecordsFns, func(mrn string, start *time.Time, end *time.Time) (*QueryResponse, error) {
		assert.Equal("123456789", mrn)
		assert.Equal(&qStart, start)
		assert.Nil(end)
		b, err := ioutil.ReadFile("./fixtures/response_success.json")
		require.NoError(err)
		var r QueryResponse
		json.Unmarshal(b, &r)
		return &r, nil
	})
	suite.hieClient.DownloadRecordFns = append(suite.hieClient.DownloadRecordFns, func(url string) (io.ReadCloser, string, error) {
		assert.Equal("http://test.foo.net/document/1.1.1.1.1.1", url)
		rc := nopCloser{bytes.NewBufferString("<foo>1</foo>")}
		return rc, "text/xml", nil
	}, func(url string) (io.ReadCloser, string, error) {
		assert.Equal("http://test.foo.net/document/1.1.1.1.1.2", url)
		rc := nopCloser{bytes.NewBufferString("<foo>2</foo>")}
		return rc, "text/xml", nil
	}, func(url string) (io.ReadCloser, string, error) {
		assert.Equal("http://test.foo.net/document/1.1.1.1.1.3", url)
		rc := nopCloser{bytes.NewBufferString("<foo>3</foo>")}
		return rc, "text/xml", nil
	})
	suite.ingestClient.IngestFns = append(suite.ingestClient.IngestFns, func(contentType string, reader io.ReadCloser) error {
		assert.Equal("text/xml", contentType)
		defer reader.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		assert.Equal("<foo>1</foo>", buf.String())
		return nil
	}, func(contentType string, reader io.ReadCloser) error {
		assert.Equal("text/xml", contentType)
		defer reader.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		assert.Equal("<foo>2</foo>", buf.String())
		return nil
	}, func(contentType string, reader io.ReadCloser) error {
		assert.Equal("text/xml", contentType)
		defer reader.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		assert.Equal("<foo>3</foo>", buf.String())
		return nil
	})
	suite.txLogMgr.FindEntriesFns = append(suite.txLogMgr.FindEntriesFns, func(ee string) ([]*TransactionLogEntry, error) {
		assert.Equal("123456789", ee)
		return []*TransactionLogEntry{
			&TransactionLogEntry{
				QueryResponseEntry: QueryResponseEntry{
					RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.0",
					CreationTime: time.Date(2007, time.March, 12, 9, 0, 0, 0, time.Local),
					Title:        "Test Continuity of Care",
					DocumentType: "XML^HL7^231^CCD^C32",
					Hash:         "1827364537281930473627184544327894736482",
					Size:         92834,
				},
				EE:   "123456789",
				Date: time.Date(2009, time.December, 31, 23, 59, 59, 0, time.Local),
			},
		}, nil
	})
	qEnd := time.Date(2016, time.June, 8, 23, 59, 59, 0, time.Local)
	suite.txLogMgr.StoreEntryFns = append(suite.txLogMgr.StoreEntryFns, func(entry *TransactionLogEntry) error {
		b, err := ioutil.ReadFile("./fixtures/response_success.json")
		require.NoError(err)
		var r QueryResponse
		json.Unmarshal(b, &r)
		assert.Equal(&TransactionLogEntry{
			QueryResponseEntry: r.Result[0],
			EE:                 "123456789",
			Date:               qEnd,
		}, entry)
		return nil
	}, func(entry *TransactionLogEntry) error {
		b, err := ioutil.ReadFile("./fixtures/response_success.json")
		require.NoError(err)
		var r QueryResponse
		json.Unmarshal(b, &r)
		assert.Equal(&TransactionLogEntry{
			QueryResponseEntry: r.Result[1],
			EE:                 "123456789",
			Date:               qEnd,
		}, entry)
		return nil
	}, func(entry *TransactionLogEntry) error {
		b, err := ioutil.ReadFile("./fixtures/response_success.json")
		require.NoError(err)
		var r QueryResponse
		json.Unmarshal(b, &r)
		assert.Equal(&TransactionLogEntry{
			QueryResponseEntry: r.Result[2],
			EE:                 "123456789",
			Date:               qEnd,
		}, entry)
		return nil
	})

	// Now just kick it all into motion
	suite.dataCopier.CopyRecords("123456789", "XML^HL7^231^CCD^C32")
}
