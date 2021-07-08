package storage

import (
	"fmt"
	"path"

	bolt "go.etcd.io/bbolt"
)

func New(dir string) (*Storage, error) {
	dbPath := path.Join(dir, "data.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database file: %w", err)
	}
	s := Storage{db}
	if err = s.initBuckets(); err != nil {
		return nil, fmt.Errorf("initilizing buckets: %w", err)
	}
	return &s, nil
}

type Storage struct {
	db *bolt.DB
}
