package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"
)

type DataCopier struct {
	hieClient    HieClient
	ingestClient IngestClient
	txLogMgr     TransactionLogManager
	pathToCopies string
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
		pathToCopies: "",
	}, nil
}

func NewDataCopierWithLocalCopies(hieClient HieClient, ingestClient IngestClient, txLogMgr TransactionLogManager, pathToCopies string) (*DataCopier, error) {
	if hieClient == nil {
		return nil, errors.New("HIE Client must be configured")
	} else if ingestClient == nil {
		return nil, errors.New("Ingest Client must be configured")
	} else if txLogMgr == nil {
		return nil, errors.New("Transaction Log Manager must be configured")
	} else if pathToCopies == "" {
		return nil, errors.New("A path to store copies must be provided")
	}

	if err := os.MkdirAll(pathToCopies, 0777); err != nil {
		return nil, err
	}

	return &DataCopier{
		hieClient:    hieClient,
		ingestClient: ingestClient,
		txLogMgr:     txLogMgr,
		pathToCopies: pathToCopies,
	}, nil
}

func (d *DataCopier) CopyRecords(mrn string, formats ...string) error {
	log.Printf("Getting transaction history for %s\n", mrn)
	history, err := d.txLogMgr.FindEntriesByEE(mrn)
	if err != nil {
		log.Printf("Error getting transaction history: %s\n", err)
		return err
	}
	log.Printf("Retrieved transaction history with %d entries\n", len(history))

	// First, take another shot at previous failed attempts
	for _, h := range history {
		if h.FailureCount > 0 {
			log.Printf("Retrying previous failed copy attempt of doc %s\n", h.DocumentID)
			if err := d.copy(h); err != nil {
				log.Printf("Failed to download document <%s> on attempt #%d: %s\n", h.DocumentID, h.FailureCount, err)
			}
			if err := d.txLogMgr.StoreEntry(h); err != nil {
				log.Printf("Failed to store log for document <%s>: %s\n", h.DocumentID, err)
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
	log.Printf("Querying records starting at %s\n", start)
	resp, err := d.hieClient.QueryRecords(mrn, &start, nil)
	if err != nil {
		log.Printf("Failed to query documents for ee %s since %s: %s\n", mrn, start.Format(time.UnixDate), err)
		return err
	}

	if !resp.Status {
		log.Printf("Unsuccessful query: %s\n", resp.Error)
		return err
	}

	// Now go through the list and copy supported documents
	if resp.Status {
		log.Printf("Query returned %d results\n", len(resp.Result))
		for _, result := range resp.Result {
			log.Printf("Processing document %s\n", result.DocumentID)
			if !supportedFormat(result.DocumentType, formats...) {
				log.Printf("Skipping due to unsupported format: %s\n", result.DocumentType)
				continue
			}
			if inHistory(result.DocumentID, history) {
				log.Printf("Skipping due to being in history\n")
				continue
			}
			// It's supported and we've never tried it before.  Attempt to copy it.
			t := TransactionLogEntry{
				QueryResponseEntry: result,
				EE:                 resp.Query.EE,
				Date:               resp.Query.EndDateTime,
			}
			if err := d.copy(&t); err != nil {
				log.Printf("Failed to download document <%s> on initial attempt: %s\n", result.DocumentID, err)
			}
			log.Printf("Storing transaction results\n")
			if err := d.txLogMgr.StoreEntry(&t); err != nil {
				log.Printf("Failed to store log for document <%s>: %s\n", result.DocumentID, err)
			}
			log.Printf("Successfully stored transaction\n")
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
	log.Printf("Downloading %s\n", t.RetrieveURL)
	rc, ct, err := d.hieClient.DownloadRecord(t.RetrieveURL)
	if err != nil {
		log.Printf("Failed download: %s\n", err.Error())
		t.Error = err.Error()
		t.FailureCount++
		return err
	}
	if d.pathToCopies != "" {
		eePath := path.Join(d.pathToCopies, t.EE)
		if err := os.MkdirAll(eePath, 0777); err != nil {
			log.Printf("Warning: Couldn't create dir %s to store copy\n", eePath)
		} else {
			filePath := path.Join(eePath, t.DocumentID+".xml")
			log.Printf("Copying to %s\n", filePath)
			// We must read out the data into a buffer first
			defer rc.Close()
			data, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}
			if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
				log.Printf("Warning: Couldn't copy to %s\n", filePath)
			}
			// Then we must reset the rc reader so the data can be uploaded
			rc = ioutil.NopCloser(bytes.NewBuffer(data))
		}
	}
	log.Printf("Uploading to ingest service w/ content type %s\n", ct)
	err = d.ingestClient.Ingest(ct, rc)
	if err != nil {
		log.Printf("Failed upload: %s\n", err.Error())
		t.Error = err.Error()
		t.FailureCount++
		return err
	}
	t.Error = ""
	t.FailureCount = 0
	log.Printf("Successful upload\n")
	return nil
}
