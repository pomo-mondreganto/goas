package storage

import (
	"github.com/boltdb/bolt"
	"strconv"
)

func (s Storage) getUserContextKey(userID int, key string) (value string, err error) {
	uk := formatUID(userID)
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		if nested := b.Bucket(uk); nested != nil {
			if data := nested.Get([]byte(key)); data != nil {
				value = string(data)
			}
		}
		return nil
	})
	return
}

func (s Storage) setUserContextKey(userID int, key string, value string) error {
	uk := formatUID(userID)
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return err
		}
		return nested.Put([]byte(key), []byte(value))
	})
}

func (s Storage) getOrSetUserContextKey(userID int, key string, value string) (result string, err error) {
	uk := formatUID(userID)
	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return err
		}
		data := nested.Get([]byte(key))
		if data != nil {
			result = string(data)
			return nil
		} else {
			result = value
			return nested.Put([]byte(key), []byte(value))
		}
	})
	return
}

func (s Storage) addToUserContextKey(userID int, key string, value int64) (result int64, err error) {
	uk := formatUID(userID)
	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return err
		}
		data := nested.Get([]byte(key))
		result = int64(0)
		if data != nil {
			if result, err = strconv.ParseInt(string(data), 10, 64); err != nil {
				return err
			}
		}
		result += value
		return nested.Put([]byte(key), []byte(strconv.FormatInt(result, 10)))
	})
	return
}

func (s Storage) addVote(vote bool, userID int, chatID int64, messageID int) error {
	bk := chatMessageVotesBucketKey(chatID, messageID)
	uid := formatUID(userID)
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(chatDataBucket))
		nested, err := b.CreateBucketIfNotExists([]byte(bk))
		if err != nil {
			return err
		}
		return nested.Put(uid, formatVote(vote))
	})
}

func (s Storage) getVoteDistribution(chatID int64, messageID int) (f int, a int, err error) {
	bk := chatMessageVotesBucketKey(chatID, messageID)
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(chatDataBucket))
		nested := b.Bucket([]byte(bk))
		if nested == nil {
			return nil
		}
		return nested.ForEach(func(_, v []byte) error {
			vote := parseVote(v)
			if vote {
				f += 1
			} else {
				a += 1
			}
			return nil
		})
	})
	return
}
