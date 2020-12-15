package storage

import (
	"fmt"
	"github.com/boltdb/bolt"
	path2 "path"
)

func New(dir string) (*Storage, error) {
	path := path2.Join(dir, "data.db")
	db, err := bolt.Open(path, 0600, nil)
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
