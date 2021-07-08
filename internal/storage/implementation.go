package storage

import (
	"fmt"
	"strconv"

	bolt "go.etcd.io/bbolt"
)

func (s Storage) getUserContextKey(userID int64, key string) (string, error) {
	uk := formatUID(userID)

	var result string
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		if nested := b.Bucket(uk); nested != nil {
			if data := nested.Get([]byte(key)); data != nil {
				result = string(data)
			}
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("executing transcation: %w", err)
	}
	return result, nil
}

func (s Storage) setUserContextKey(userID int64, key string, value string) error {
	uk := formatUID(userID)

	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return fmt.Errorf("creating bucket %v: %w", uk, err)
		}
		if err := nested.Put([]byte(key), []byte(value)); err != nil {
			return fmt.Errorf("setting user's %v key %v: %w", userID, key, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("executing transcation: %w", err)
	}
	return nil
}

func (s Storage) getOrSetUserContextKey(userID int64, key string, value string) (string, error) {
	uk := formatUID(userID)

	var result string
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return fmt.Errorf("creating bucket %v: %w", uk, err)
		}
		data := nested.Get([]byte(key))
		if data != nil {
			result = string(data)
			return nil
		} else {
			result = value
			if err := nested.Put([]byte(key), []byte(value)); err != nil {
				return fmt.Errorf("setting user's %v key %v: %w", userID, key, err)
			}
			return nil
		}
	}); err != nil {
		return "", fmt.Errorf("executing transcation: %w", err)
	}
	return result, nil
}

func (s Storage) addToUserContextKey(userID int64, key string, value int64) (int64, error) {
	uk := formatUID(userID)

	var result int64
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userDataBucket))
		nested, err := b.CreateBucketIfNotExists(uk)
		if err != nil {
			return fmt.Errorf("creating bucket %v: %w", uk, err)
		}
		data := nested.Get([]byte(key))
		result = int64(0)
		if data != nil {
			if result, err = strconv.ParseInt(string(data), 10, 64); err != nil {
				return fmt.Errorf("parsing old key's %v value (%v): %w", key, string(data), err)
			}
		}
		result += value
		if err := nested.Put([]byte(key), []byte(strconv.FormatInt(result, 10))); err != nil {
			return fmt.Errorf("setting user's %v key %v: %w", userID, key, err)
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("executing transcation: %w", err)
	}
	return result, nil
}

func (s Storage) addVote(vote bool, userID int64, chatID int64, messageID int) error {
	bk := chatMessageVotesBucketKey(chatID, messageID)
	uid := formatUID(userID)
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(chatDataBucket))
		nested, err := b.CreateBucketIfNotExists([]byte(bk))
		if err != nil {
			return fmt.Errorf("creaing bucket %v: %w", bk, err)
		}
		if err := nested.Put(uid, formatVote(vote)); err != nil {
			return fmt.Errorf("setting vote for %v: %w", uid, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("executing transaction: %w", err)
	}
	return nil
}

func (s Storage) getVotes(chatID int64, messageID int) (f int, a int, err error) {
	bk := chatMessageVotesBucketKey(chatID, messageID)
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(chatDataBucket))
		nested := b.Bucket([]byte(bk))
		if nested == nil {
			return nil
		}
		if err := nested.ForEach(func(_, v []byte) error {
			vote := parseVote(v)
			if vote {
				f += 1
			} else {
				a += 1
			}
			return nil
		}); err != nil {
			return fmt.Errorf("iterating votes bucket: %w", err)
		}
		return nil
	})
	return
}
