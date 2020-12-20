package storage

import (
	"bytes"
	"fmt"
	"strconv"
)

func intToBytes(i int) []byte {
	return []byte(strconv.FormatInt(int64(i), 10))
}

func formatUID(userID int) []byte {
	return intToBytes(userID)
}

func chatMessageCountKey(chatID int64) string {
	return fmt.Sprintf("chat:%d:msg_count", chatID)
}

func chatMessageKey(chatID int64, messageID int) string {
	return fmt.Sprintf("chat_msg:%d:%d", chatID, messageID)
}

func chatMessageVotesBucketKey(chatID int64, messageID int) string {
	return fmt.Sprintf("%s:votes", chatMessageKey(chatID, messageID))
}

func formatVote(vote bool) []byte {
	if vote {
		return []byte("1")
	} else {
		return []byte("0")
	}
}

func parseVote(data []byte) bool {
	if bytes.Equal(data, []byte("1")) {
		return true
	}
	return false
}
