package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/dbtest"

	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestTxLogManagerSuite(t *testing.T) {
	suite.Run(t, new(TxLogManagerSuite))
}

type TxLogManagerSuite struct {
	suite.Suite
	DBServer         *dbtest.DBServer
	DBServerPath     string
	Session          *mgo.Session
	Database         *mgo.Database
	TxLogMgr         *MgoTransactionLogManager
	HIEResultEntries []QueryResponseEntry
}

func (suite *TxLogManagerSuite) SetupSuite() {
	require := suite.Require()

	suite.DBServer = &dbtest.DBServer{}
	var err error
	suite.DBServerPath, err = ioutil.TempDir("", "mongotestdb")
	require.NoError(err)
	suite.DBServer.SetPath(suite.DBServerPath)
}

func (suite *TxLogManagerSuite) SetupTest() {
	require := suite.Require()

	suite.Session = suite.DBServer.Session()
	suite.Database = suite.Session.DB("redcap-riskservice-test")
	var err error
	suite.TxLogMgr, err = NewMgoTransactionLogManager(suite.Database)
	require.NoError(err)

	b, err := ioutil.ReadFile("./fixtures/response_success.json")
	require.NoError(err)
	var r QueryResponse
	err = json.Unmarshal(b, &r)
	require.NoError(err)
	suite.HIEResultEntries = r.Result
}

func (suite *TxLogManagerSuite) TearDownTest() {
	suite.Session.Close()
	suite.DBServer.Wipe()
	suite.HIEResultEntries = []QueryResponseEntry{}
}

func (suite *TxLogManagerSuite) TearDownSuite() {
	suite.DBServer.Stop()
	if err := os.RemoveAll(suite.DBServerPath); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error cleaning up temp directory: %s", err.Error())
	}
}

func (suite *TxLogManagerSuite) TestStoreEntry() {
	assert := suite.Assert()
	require := suite.Require()

	// First store an error
	entry := &TransactionLogEntry{
		QueryResponseEntry: suite.HIEResultEntries[0],
		EE:                 "123456789",
		Error:              "Not Found",
		FailureCount:       1,
		Date:               time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
	}

	err := suite.TxLogMgr.StoreEntry(entry)
	require.NoError(err)

	// Now look into the DB and ensure it's what we think
	var actual TransactionLogEntry
	suite.Database.C("transactions").FindId("1.1.1.1.1.1").One(&actual)
	assert.Equal(TransactionLogEntry{
		QueryResponseEntry: QueryResponseEntry{
			RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.1",
			CreationTime: time.Date(2014, 4, 25, 2, 51, 3, 0, time.Local),
			Title:        "Test Continuity of Care",
			DocumentType: "XML^HL7^231^CCD^C32",
			DocumentID:   "1.1.1.1.1.1",
			Hash:         "4C167E7B7F006A18ABB2E4A1A9B2489936947E91",
			Size:         28452,
		},
		EE:           "123456789",
		Error:        "Not Found",
		FailureCount: 1,
		Date:         time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
	}, actual)

	// And now lets update it with a success!
	actual.Error = ""
	actual.FailureCount = 0

	err = suite.TxLogMgr.StoreEntry(&actual)
	require.NoError(err)

	var actual2 TransactionLogEntry
	suite.Database.C("transactions").FindId("1.1.1.1.1.1").One(&actual2)
	assert.Equal(TransactionLogEntry{
		QueryResponseEntry: QueryResponseEntry{
			RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.1",
			CreationTime: time.Date(2014, 4, 25, 2, 51, 3, 0, time.Local),
			Title:        "Test Continuity of Care",
			DocumentType: "XML^HL7^231^CCD^C32",
			DocumentID:   "1.1.1.1.1.1",
			Hash:         "4C167E7B7F006A18ABB2E4A1A9B2489936947E91",
			Size:         28452,
		},
		EE:           "123456789",
		Error:        "",
		FailureCount: 0,
		Date:         time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
	}, actual2)
}

func (suite *TxLogManagerSuite) TestStoreEntryWithNoDocumentID() {
	assert := suite.Assert()

	entry := &TransactionLogEntry{
		QueryResponseEntry: suite.HIEResultEntries[0],
		EE:                 "123456789",
		Error:              "Not Found",
		FailureCount:       1,
		Date:               time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
	}
	entry.DocumentID = ""

	err := suite.TxLogMgr.StoreEntry(entry)
	assert.Error(err)
}

func (suite *TxLogManagerSuite) TestFindEntriesByEE() {
	assert := suite.Assert()
	require := suite.Require()

	// Assume that store works (since we test it elsewhere) to setup the test
	for i, result := range suite.HIEResultEntries {
		entry := TransactionLogEntry{
			QueryResponseEntry: result,
			EE:                 "123456789",
			Error:              "Not Found",
			FailureCount:       i + 1,
			Date:               time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
		}
		err := suite.TxLogMgr.StoreEntry(&entry)
		require.NoError(err)
	}

	// Throw in one more w/ different EE just to make sure the filtering on EE is happening
	entry := TransactionLogEntry{
		QueryResponseEntry: QueryResponseEntry{
			RetrieveURL:  "http://test.foo.net/document/2.2.2.2.2.2",
			CreationTime: time.Date(2014, 4, 26, 3, 15, 0, 0, time.Local),
			Title:        "Test Continuity of Care",
			DocumentType: "XML^HL7^231^CCD^C32",
			DocumentID:   "2.2.2.2.2.2",
			Hash:         "928371EDF671AB03CD40912DE83F7AE7888DDEF",
			Size:         36589,
		},
		EE:           "987654321",
		Error:        "",
		FailureCount: 0,
		Date:         time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
	}
	err := suite.TxLogMgr.StoreEntry(&entry)
	require.NoError(err)

	// First just test the one
	entries, err := suite.TxLogMgr.FindEntriesByEE("987654321")
	require.NoError(err)
	assert.Len(entries, 1)
	assert.Equal(&entry, entries[0])

	// Then test the three
	entries, err = suite.TxLogMgr.FindEntriesByEE("123456789")
	require.NoError(err)
	assert.Len(entries, 3)

	// Just check the middle one
	assert.Equal(&TransactionLogEntry{
		QueryResponseEntry: QueryResponseEntry{
			RetrieveURL:  "http://test.foo.net/document/1.1.1.1.1.2",
			CreationTime: time.Date(2014, 4, 25, 2, 14, 3, 0, time.Local),
			Title:        "Test Continuity of Care",
			DocumentType: "XML^HL7^231^CCD^C32",
			DocumentID:   "1.1.1.1.1.2",
			Hash:         "5B885732FE2D9D33AAEBBDA3CCE01A2F1D279E13",
			Size:         27869,
		},
		EE:           "123456789",
		Error:        "Not Found",
		FailureCount: 2,
		Date:         time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
	}, entries[1])
}

func (suite *TxLogManagerSuite) TestFindEntriesByInvalidEE() {
	assert := suite.Assert()
	require := suite.Require()

	// Assume that store works (since we test it elsewhere) to setup the test
	for i, result := range suite.HIEResultEntries {
		entry := TransactionLogEntry{
			QueryResponseEntry: result,
			EE:                 "123456789",
			Error:              "Not Found",
			FailureCount:       i + 1,
			Date:               time.Date(2016, time.June, 12, 3, 0, 14, 0, time.Local),
		}
		err := suite.TxLogMgr.StoreEntry(&entry)
		require.NoError(err)
	}

	entries, err := suite.TxLogMgr.FindEntriesByEE("ABCDEFGHIJK")
	require.NoError(err)
	assert.Empty(entries)
}
