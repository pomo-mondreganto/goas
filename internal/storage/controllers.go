package storage

import (
	"fmt"
	"strconv"
	"time"
)

const (
	trustKey     = "trusted"
	firstSeenKey = "first_seen"

	trustValue = "yes"
)

func (s Storage) IsUserAdmin(userID int64) bool {
	return userID == 143994885 || userID == 167389904
}

func (s Storage) TrustUser(userID int64) error {
	return s.setUserContextKey(userID, trustKey, trustValue)
}

func (s Storage) IsUserTrusted(userID int64) (bool, error) {
	value, err := s.getUserContextKey(userID, trustKey)
	if err != nil {
		return false, err
	}
	return value == trustValue, nil
}

func (s Storage) GetOrSetUserFirstSeen(userID int64, firstSeen time.Time) (time.Time, error) {
	formatted := strconv.FormatInt(firstSeen.UnixNano(), 10)
	value, err := s.getOrSetUserContextKey(userID, firstSeenKey, formatted)
	if err != nil {
		return time.Time{}, fmt.Errorf("get pr set user's %v first seen: %w", userID, err)
	}
	nano, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing first seen (%v): %w", value, err)
	}
	return time.Unix(0, nano), nil
}

func (s Storage) IncUserChatMessageCount(userID int64, chatID int64, add int64) (int64, error) {
	key := chatMessageCountKey(chatID)
	val, err := s.addToUserContextKey(userID, key, add)
	if err != nil {
		return 0, fmt.Errorf("incrementing user (%v) context key %v: %w", userID, key, err)
	}
	return val, nil
}

func (s Storage) GetUserChatMessageCount(userID int64, chatID int64) (int64, error) {
	return s.IncUserChatMessageCount(userID, chatID, 0)
}

func (s Storage) VoteSpam(userID int64, chatID int64, messageID int, spam bool) error {
	return s.addVote(spam, userID, chatID, messageID)
}

func (s Storage) GetVotes(chatID int64, messageID int) (int, int, error) {
	return s.getVotes(chatID, messageID)
}
