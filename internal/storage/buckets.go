package storage

import (
	"github.com/boltdb/bolt"
	"github.com/sirupsen/logrus"
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
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range bucketNames {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		return nil
	})
}
