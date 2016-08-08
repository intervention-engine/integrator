package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type HieClient interface {
	QueryRecords(mrn string, start *time.Time, end *time.Time) (*QueryResponse, error)
	DownloadRecord(url string) (content io.ReadCloser, contentType string, err error)
}

type HttpHieClient struct {
	BaseURL      string
	UseCUrl      bool
	UseBasicAuth bool
	User         string
	Password     string
}

func NewHttpHieClient(baseURL string) *HttpHieClient {
	return &HttpHieClient{
		BaseURL: baseURL,
	}
}

func NewBasicAuthHttpHieClient(baseURL, user, password string) *HttpHieClient {
	return &HttpHieClient{
		BaseURL:      baseURL,
		UseBasicAuth: true,
		User:         user,
		Password:     password,
	}
}

func NewCUrlHttpHieClient(baseURL string) *HttpHieClient {
	c := NewHttpHieClient(baseURL)
	c.UseCUrl = true
	return c
}

func NewCUrlBasicAuthHttpHieClient(baseURL, user, password string) *HttpHieClient {
	c := NewBasicAuthHttpHieClient(baseURL, user, password)
	c.UseCUrl = true
	return c
}

func (c *HttpHieClient) QueryRecords(mrn string, start *time.Time, end *time.Time) (*QueryResponse, error) {
	params := url.Values{}
	params.Set("ee", mrn)
	if start != nil {
		params.Set("startDateTime", start.Format("2006-01-02T15:04:05"))
	}
	if end != nil {
		params.Set("endDateTime", end.Format("2006-01-02T15:04:05"))
	}

	qURL := c.BaseURL + "?" + params.Encode()
	var data io.ReadCloser
	var err error
	var cmd *exec.Cmd
	if c.UseCUrl {
		args := []string{}
		if c.UseBasicAuth {
			args = append(args, "-u", c.User+":"+c.Password)
		}
		args = append(args, qURL)
		cmd = exec.Command("curl", args...)
		data, err = cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
	} else {
		req, err2 := http.NewRequest("GET", qURL, nil)
		if err2 != nil {
			return nil, err2
		}
		if c.UseBasicAuth {
			req.SetBasicAuth(c.User, c.Password)
		}
		resp, err2 := http.DefaultClient.Do(req)
		if err2 != nil {
			return nil, err2
		}
		data = resp.Body
		defer data.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("Non-OK response from source server: %d (%s)", resp.StatusCode, resp.Status)
		}
	}

	qr := new(QueryResponse)
	if err := json.NewDecoder(data).Decode(qr); err != nil {
		return nil, err
	}

	if c.UseCUrl {
		if err := cmd.Wait(); err != nil {
			return nil, err
		}
	}

	return qr, err
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func (c *HttpHieClient) DownloadRecord(url string) (content io.ReadCloser, contentType string, err error) {
	if c.UseCUrl {
		args := []string{}
		if c.UseBasicAuth {
			args = append(args, "-u", c.User+":"+c.Password)
		}
		args = append(args, url)
		data, err := exec.Command("curl", args...).Output()
		if err != nil {
			return nil, "", err
		}
		// Not ideal, but for now, we're assuming it's always XML.
		return nopCloser{bytes.NewBuffer(data)}, "text/xml; charset=utf-8", nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	if c.UseBasicAuth {
		req.SetBasicAuth(c.User, c.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}

	// If the status code is an error, we don't have a doc -- we have a JSON response with an error.
	// Parse it and return the error.
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		qr := new(QueryResponse)
		if err := json.NewDecoder(resp.Body).Decode(qr); err != nil {
			return nil, "", err
		}
		return nil, "", errors.New(qr.Error)
	}

	// Request was successful, so just pass along the body and content type
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// QueryResponse represents the response for a query to the HIE
type QueryResponse struct {
	Status bool                 `json:"status"`
	Result []QueryResponseEntry `json:"result"`
	Error  string               `json:"error"`
	Query  QueryRequest         `json:"query"`
}

// QueryResponseEntry represents an entry in the query response
type QueryResponseEntry struct {
	RetrieveURL  string    `json:"retrieveURL"`
	CreationTime time.Time `json:"creationTime"`
	Title        string    `json:"title"`
	DocumentType string    `json:"documentType"`
	DocumentID   string    `json:"documentID"`
	Hash         string    `json:"hash"`
	Size         int       `json:"size"`
}

// UnmarshalJSON contains custom unmarshaling logic to handle the incoming date formats
func (q *QueryResponseEntry) UnmarshalJSON(data []byte) (err error) {
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	q.RetrieveURL = m["retrieveURL"].(string)
	q.Title = m["title"].(string)
	q.DocumentType = m["documentType"].(string)
	q.DocumentID = m["documentID"].(string)
	q.Hash = m["hash"].(string)
	q.Size = int(m["size"].(float64))
	if err != nil {
		return err
	}
	q.CreationTime, err = time.ParseInLocation("20060102150405", m["creationTime"].(string), time.Local)
	if err != nil {
		return err
	}
	return nil
}

// QueryRequest represents the details of the query sent to the HIE
type QueryRequest struct {
	Env                   string    `json:"env"`
	Host                  string    `json:"host"`
	EE                    string    `json:"ee"`
	StartDateTime         time.Time `json:"startDateTime"`
	EndDateTime           time.Time `json:"endDateTime"`
	QueryStartDateTime    time.Time `json:"queryStartDateTime"`
	QueryCompleteDateTime time.Time `json:"queryCompleteEndDateTime"`
}

// UnmarshalJSON contains custom unmarshaling logic to handle the incoming date formats
func (q *QueryRequest) UnmarshalJSON(data []byte) (err error) {
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	q.Env = m["env"]
	q.Host = m["host"]
	q.EE = m["ee"]
	q.StartDateTime, err = time.ParseInLocation("2006-01-02T15:04:05", m["startDateTime"], time.Local)
	if err != nil {
		return err
	}
	q.EndDateTime, err = time.ParseInLocation("2006-01-02T15:04:05", m["endDateTime"], time.Local)
	if err != nil {
		return err
	}
	q.QueryStartDateTime, err = lenientParse("2006-01-02T15:04:05.000000000Z", m["queryStartDateTime"])
	if err != nil {
		return err
	}
	q.QueryCompleteDateTime, err = lenientParse("2006-01-02T15:04:05.000000000Z", m["queryCompleteDateTime"])
	if err != nil {
		return err
	}
	return nil
}

func lenientParse(dtfmt, dt string) (time.Time, error) {
	t, err := time.Parse(dtfmt, dt)
	if err != nil && strings.Contains(dtfmt, "0Z") {
		dtfmt = strings.Replace(dtfmt, "0Z", "Z", 1)
		return lenientParse(dtfmt, dt)
	}
	return t, err
}
