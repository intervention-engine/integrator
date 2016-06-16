package main

import (
	"fmt"
	"io"
	"net/http"
)

type IngestClient interface {
	Ingest(contentType string, reader io.ReadCloser) error
}

type HttpIngestClient struct {
	BaseURL string
}

func NewHttpIngestClient(baseURL string) *HttpIngestClient {
	return &HttpIngestClient{
		BaseURL: baseURL,
	}
}

func (i *HttpIngestClient) Ingest(contentType string, reader io.ReadCloser) error {
	resp, err := http.DefaultClient.Post(i.BaseURL, contentType, reader)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to post content.  Received %d: %s", resp.StatusCode, resp.Status)
	}
	return nil
}
