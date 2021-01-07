package main

import (
	"fmt"
	"gitlab.com/sapienzastudents/antispam-telegram-bot/botdatabase"
	tb "gopkg.in/tucnak/telebot.v2"
	"reflect"
	"sort"
	"strings"
	"time"
)

var botPermissionsTag = map[string]string{
	"can_change_info":      "C",
	"can_delete_messages":  "D",
	"can_invite_users":     "I",
	"can_restrict_members": "R",
	"can_pin_messages":     "N",
	"can_promote_members":  "P",
}

var botPermissionsText = map[string]string{
	"can_change_info":      "Change group info",
	"can_delete_messages":  "Delete messages",
	"can_invite_users":     "Invite users via link",
	"can_restrict_members": "Restrict/ban users",
	"can_pin_messages":     "Pin messages",
	"can_promote_members":  "Add new admins",
}

func onGroupsPrivileges(m *tb.Message, _ botdatabase.ChatSettings) {
	onGroupsPrivilegesFunc(m, false)
}

func onGroupsNotifyMissingPermissions(m *tb.Message, _ botdatabase.ChatSettings) {
	onGroupsPrivilegesFunc(m, true)
}

func synthetizePrivileges(user *tb.ChatMember) []string {
	var ret []string
	t := reflect.TypeOf(user.Rights)
	right := reflect.ValueOf(user.Rights)
	for i := 0; i < t.NumField(); i++ {
		k := t.Field(i).Tag.Get("json")
		_, ok := botPermissionsTag[k]
		if !ok {
			// Skip this field
			continue
		}

		f := right.Field(i)
		if !f.Bool() {
			ret = append(ret, k)
		}
	}
	return ret
}

func onGroupsPrivilegesFunc(m *tb.Message, notify bool) {
	if notify {
		logger.Debugf("Missing privilege notification requested by %d (%s %s %s)", m.Sender.ID, m.Sender.Username, m.Sender.FirstName, m.Sender.LastName)
	} else {
		logger.Debugf("My chat room privileges requested by %d (%s %s %s)", m.Sender.ID, m.Sender.Username, m.Sender.FirstName, m.Sender.LastName)
	}

	waitingmsg, _ := b.Send(m.Chat, "Work in progress...")

	chatrooms, err := botdb.ListMyChatrooms()
	if err != nil {
		logger.WithError(err).Error("Error getting chatroom list")
	} else {
		sort.Slice(chatrooms, func(i, j int) bool {
			return chatrooms[i].Title < chatrooms[j].Title
		})

		msg := strings.Builder{}
		for k, v := range botPermissionsTag {
			msg.WriteString(k)
			msg.WriteString(" -> ")
			msg.WriteString(v)
			msg.WriteString("\n")
		}
		msg.WriteString("\n")

		for _, v := range chatrooms {
			newInfos, err := b.ChatByID(fmt.Sprint(v.ID))
			if err != nil {
				logger.Warning("can't get refreshed infos for chatroom ", v, " ", err)
				continue
			}
			v = newInfos

			me, err := b.ChatMemberOf(v, b.Me)
			if err != nil {
				logger.Warning("can't get refreshed infos for chatroom ", v, " ", err)
				continue
			}

			msg.WriteString(" - ")
			msg.WriteString(v.Title)
			msg.WriteString(" : ")
			if me.Role != tb.Administrator {
				msg.WriteString("❌ not admin\n")

				if notify {
					_, _ = b.Send(v, "Oops, mi mancano i permessi di admin per funzionare! L'indicizzazione non sta funzionando!\n\nPer gli admin del gruppo: contattatemi in privato scrivendo /settings per vedere quali permessi mancano")
				}
			} else {
				var missingPrivileges = synthetizePrivileges(me)
				if len(missingPrivileges) == 0 {
					msg.WriteString("✅\n")
				} else {
					for _, k := range missingPrivileges {
						msg.WriteString(botPermissionsTag[k])
					}
					msg.WriteString("❌\n")

					if notify {
						_, _ = b.Send(v, "Oops, mi mancano alcuni permessi per funzionare!\n\nPer gli admin del gruppo: contattatemi in privato scrivendo /settings per vedere quali permessi mancano")
					}
				}
			}

			_, err = b.Edit(waitingmsg, msg.String())
			if err != nil {
				logger.Warning("[global] can't edit message to the user ", err)
			}

			// Do not trigger Telegram rate limit
			time.Sleep(500 * time.Millisecond)
		}

		msg.WriteString("\ndone")

		_, err = b.Edit(waitingmsg, msg.String())
		if err != nil {
			logger.Warning("[global] can't edit final message to the user ", err)
		}
	}
}
