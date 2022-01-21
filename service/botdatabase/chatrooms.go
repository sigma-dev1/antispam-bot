package botdatabase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"
	tb "gopkg.in/tucnak/telebot.v3"
)

// AddOrUpdateChat adds or updates the chat info into the DB.
//
// As Telegram doesn't offer a way to track in which chatrooms the bot is, we
// need to store it in Redis.
//
// Time complexity: O(1).
func (db *_botDatabase) AddOrUpdateChat(c *tb.Chat) error {
	// Do not marshal PinnedMessage because it cannot be unmarshalled (there is
	// a bug in the telebot library).
	c.PinnedMessage = nil

	jsonbin, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return db.redisconn.HSet(context.TODO(), "chatrooms", strconv.FormatInt(c.ID, 10), string(jsonbin)).Err()
}

// DeleteChat removes all chatroom info of the given chat ID.
//
// Time complexity: O(1).
func (db *_botDatabase) DeleteChat(chatID int64) error {
	// DeleteChat works by removing the named field in sets: "public-links",
	// "settings" and "chatrooms".
	err := db.redisconn.HDel(context.TODO(), "public-links", strconv.FormatInt(chatID, 10)).Err()
	if err != nil {
		return err
	}
	err = db.redisconn.HDel(context.TODO(), "settings", strconv.FormatInt(chatID, 10)).Err()
	if err != nil {
		return err
	}
	return db.redisconn.HDel(context.TODO(), "chatrooms", strconv.FormatInt(chatID, 10)).Err()
}

// ChatroomsCount returns the count of chatrooms where the bot is.
//
// Time complexity: O(1).
func (db *_botDatabase) ChatroomsCount() (int64, error) {
	ret, err := db.redisconn.HLen(context.TODO(), "chatrooms").Result()
	if err == redis.Nil {
		return 0, nil
	}
	return ret, err
}

// ListMyChatrooms returns the list of chatrooms where the bot is.
//
// Time complexity: O(n) where "n" is the number of chat where the bot is.
func (db *_botDatabase) ListMyChatrooms() ([]*tb.Chat, error) {
	var chatrooms []*tb.Chat

	var cursor uint64 = 0
	var err error
	var keys []string
	// ListMyChatrooms works by by deserializing the tb.Chat for each chatroom.
	for {
		keys, cursor, err = db.redisconn.HScan(context.TODO(), "chatrooms", cursor, "", -1).Result()
		if err == redis.Nil {
			return chatrooms, nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to scan chatrooms: %w", err)
		}

		for i := 0; i < len(keys); i += 2 {
			// ChatTmp is a partial copy of telebot.Chat, because some field
			// marshalled into JSON cannot be unmarhalled, there is an error in
			// the library. This is a temporary fix.
			type ChatTmp struct {
				ID        int64       `json:"id"`
				Type      tb.ChatType `json:"type"`
				Title     string      `json:"title"`
				FirstName string      `json:"first_name"`
				LastName  string      `json:"last_name"`
				Username  string      `json:"username"`
				Still     bool        `json:"is_member,omitempty"`
			}

			chat := ChatTmp{}
			err = json.Unmarshal([]byte(keys[i+1]), &chat)
			if err != nil {
				return nil, fmt.Errorf("failed to scan chatroom %q: %w", keys[i+1], err)
			}

			room := tb.Chat{
				ID:        chat.ID,
				Type:      chat.Type,
				Title:     chat.Title,
				FirstName: chat.FirstName,
				LastName:  chat.LastName,
				Username:  chat.Username,
				Still:     chat.Still,
			}
			chatrooms = append(chatrooms, &room)
		}

		// SCAN cycle end
		if cursor == 0 {
			break
		}
	}

	return chatrooms, nil
}
