package storage

import (
	"fmt"
	"github.com/boltdb/bolt"
	"strconv"
)

func userKey(userID int) []byte {
	return []byte(strconv.FormatInt(int64(userID), 10))
}

func chatMessagesKey(chatID int64) string {
	return fmt.Sprintf("chat:%d:msg_count", chatID)
}

func (s Storage) getUserContextKey(userID int, key string) (value string, err error) {
	uk := userKey(userID)
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
	uk := userKey(userID)
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
	uk := userKey(userID)
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
	uk := userKey(userID)
	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return err
		}
		data := nested.Get([]byte(key))
		result = int64(0)
		if data != nil {
			result, err = strconv.ParseInt(string(data), 10, 64)
		}
		result += value
		return nested.Put([]byte(key), []byte(strconv.FormatInt(result, 10)))
	})
	return
}
