package storage

import (
	"strconv"
	"time"
)

const (
	trustKey     = "trusted"
	firstSeenKey = "first_seen"

	trustValue = "yes"
)

func (s Storage) IsUserAdmin(userID int) bool {
	return userID == 143994885 || userID == 167389904
}

func (s Storage) TrustUser(userID int) error {
	return s.setUserContextKey(userID, trustKey, trustValue)
}

func (s Storage) IsUserTrusted(userID int) (bool, error) {
	value, err := s.getUserContextKey(userID, trustKey)
	if err != nil {
		return false, err
	}
	return value == trustValue, nil
}

func (s Storage) GetOrSetUserFirstSeen(userID int, firstSeen time.Time) (time.Time, error) {
	formatted := strconv.FormatInt(firstSeen.UnixNano(), 10)
	value, err := s.getOrSetUserContextKey(userID, firstSeenKey, formatted)
	if err != nil {
		return time.Time{}, err
	}
	nano, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, nano), nil
}

func (s Storage) IncUserChatMessageCount(userID int, chatID int64, add int64) (int64, error) {
	key := chatMessageCountKey(chatID)
	return s.addToUserContextKey(userID, key, add)
}

func (s Storage) GetUserChatMessageCount(userID int, chatID int64) (int64, error) {
	key := chatMessageCountKey(chatID)
	return s.addToUserContextKey(userID, key, 0)
}

func (s Storage) VoteSpam(userID int, chatID int64, messageID int, spam bool) error {
	return s.addVote(spam, userID, chatID, messageID)
}

func (s Storage) GetVotes(chatID int64, messageID int) (int, int, error) {
	return s.getVoteDistribution(chatID, messageID)
}
