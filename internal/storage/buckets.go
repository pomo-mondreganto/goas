package storage

import (
	"fmt"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	userDataBucket = "users"
	chatDataBucket = "chats"
)

var bucketNames = []string{
	userDataBucket,
	chatDataBucket,
}

func (s Storage) initBuckets() error {
	logrus.Info("Initializing buckets")
	if err := s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range bucketNames {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("creating bucket %v: %w", name, err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("executing transaction: %w", err)
	}
	return nil
}
