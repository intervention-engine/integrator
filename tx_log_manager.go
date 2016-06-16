package main

import (
	"errors"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type TransactionLogEntry struct {
	QueryResponseEntry `bson:",inline"`
	EE                 string    `bson:"ee"`
	Error              string    `bson:"error,omitempty"`
	FailureCount       int       `bson:"failureCount"`
	Date               time.Time `bson:"date"`
}

type TransactionLogManager interface {
	FindEntriesByEE(ee string) (entries []*TransactionLogEntry, err error)
	StoreEntry(entry *TransactionLogEntry) error
}

type MgoTransactionLogManager struct {
	txCollection *mgo.Collection
}

func NewMgoTransactionLogManager(db *mgo.Database) (*MgoTransactionLogManager, error) {
	if db == nil || db.Session == nil {
		return nil, errors.New("The Mongo DB must be configured")
	}

	return &MgoTransactionLogManager{
		txCollection: db.C("transactions"),
	}, nil
}

func (t *MgoTransactionLogManager) FindEntriesByEE(ee string) (entries []*TransactionLogEntry, err error) {
	if t.txCollection == nil {
		return nil, errors.New("The transaction database collection is not configured")
	}
	entries = []*TransactionLogEntry{}
	if err := t.txCollection.Find(bson.M{"ee": ee}).All(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (t *MgoTransactionLogManager) StoreEntry(entry *TransactionLogEntry) error {
	if t.txCollection == nil {
		return errors.New("The transaction database collection is not configured")
	} else if entry.DocumentID == "" {
		return errors.New("Cannot store a transaction without a valid document ID")
	}
	_, err := t.txCollection.UpsertId(entry.DocumentID, entry)
	return err
}
