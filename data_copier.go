package main

import (
	"errors"
	"fmt"
	"time"
)

type DataCopier struct {
	hieClient    HieClient
	ingestClient IngestClient
	txLogMgr     TransactionLogManager
}

func NewDataCopier(hieClient HieClient, ingestClient IngestClient, txLogMgr TransactionLogManager) (*DataCopier, error) {
	if hieClient == nil {
		return nil, errors.New("HIE Client must be configured")
	} else if ingestClient == nil {
		return nil, errors.New("Ingest Client must be configured")
	} else if txLogMgr == nil {
		return nil, errors.New("Transaction Log Manager must be configured")
	}
	return &DataCopier{
		hieClient:    hieClient,
		ingestClient: ingestClient,
		txLogMgr:     txLogMgr,
	}, nil
}

func (d *DataCopier) CopyRecords(mrn string, formats ...string) error {
	history, err := d.txLogMgr.FindEntriesByEE(mrn)
	if err != nil {
		return err
	}

	// First, take another shot at previous failed attempts
	for _, h := range history {
		if h.FailureCount > 0 {
			if err := d.copy(h); err != nil {
				fmt.Printf("Failed to download document <%s> on attempt #%d: %s\n", h.DocumentID, h.FailureCount, err)
			}
			if err := d.txLogMgr.StoreEntry(h); err != nil {
				fmt.Printf("Failed to store log for document <%s>: %s\n", h.DocumentID, err)
			}
		}
	}

	// Now determine the start date for the query to the HIE
	start := time.Date(1900, time.January, 1, 0, 0, 0, 0, time.Local)
	for _, h := range history {
		if !h.Date.Before(start) {
			// Add one second since the date is inclusive in the last query
			start = h.Date.Add(1 * time.Second)
		}
	}

	// Query for the document list
	resp, err := d.hieClient.QueryRecords(mrn, &start, nil)
	if err != nil {
		fmt.Printf("Failed to query documents for ee %s since %s: %s\n", mrn, start.Format(time.UnixDate), err)
	}

	// Now go through the list and copy supported documents
	if resp.Status {
		for _, result := range resp.Result {
			if !supportedFormat(result.DocumentType, formats...) {
				continue
			}
			if inHistory(result.DocumentID, history) {
				continue
			}
			// It's supported and we've never tried it before.  Attempt to copy it.
			t := TransactionLogEntry{
				QueryResponseEntry: result,
				EE:                 resp.Query.EE,
				Date:               resp.Query.EndDateTime,
			}
			if err := d.copy(&t); err != nil {
				fmt.Printf("Failed to download document <%s> on initial attempt: %s\n", result.DocumentID, err)
			}
			if err := d.txLogMgr.StoreEntry(&t); err != nil {
				fmt.Printf("Failed to store log for document <%s>: %s\n", result.DocumentID, err)
			}
		}
	}
	return nil
}

func supportedFormat(fmt string, supportedFmts ...string) bool {
	for _, supportFmt := range supportedFmts {
		if fmt == supportFmt {
			return true
		}
	}
	return false
}

func inHistory(documentID string, history []*TransactionLogEntry) bool {
	for _, h := range history {
		if documentID == h.DocumentID {
			return true
		}
	}
	return false
}

func (d *DataCopier) copy(t *TransactionLogEntry) error {
	rc, ct, err := d.hieClient.DownloadRecord(t.RetrieveURL)
	if err != nil {
		t.Error = err.Error()
		t.FailureCount++
		return err
	}
	err = d.ingestClient.Ingest(ct, rc)
	if err != nil {
		t.Error = err.Error()
		t.FailureCount++
		return err
	}
	return nil
}
