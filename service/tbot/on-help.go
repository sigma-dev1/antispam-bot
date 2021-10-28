package tbot

import (
	"strings"

	"github.com/google/uuid"
	tb "gopkg.in/tucnak/telebot.v3"
)

// startFromUUID replies to the user with the invite link of a chat from a UUID. The UUID is used in the website to
// avoid SPAM bots. All links in the website will cause the user to send a "/start UUID" message
func (bot *telegramBot) startFromUUID(payload string, sender *tb.User) {
	chatUUID, err := uuid.Parse(payload)
	if err != nil {
		bot.logger.WithError(err).Error("error parsing chat UUID")
		return
	}

	chatID, err := bot.db.GetChatIDFromUUID(chatUUID)
	if err != nil {
		bot.logger.WithError(err).Error("chat not found for UUID " + chatUUID.String())
		return
	}

	var msg string

	inviteLink, err := bot.getInviteLink(&tb.Chat{ID: chatID})
	if err != nil {
		bot.logger.WithError(err).WithField("chat", chatID).Warning("can't generate invite link")
		msg = "Ooops, ho perso qualche rotella, avverti il mio admin che mi sono rotto :-("
	} else {
		msg = "🇮🇹 Ciao! Il link di invito è questo qui sotto (se dice che non è funzionante, riprova ad usarlo tra 1-2 minuti):\n\n🇬🇧 Hi! The invite link is the following (if Telegram says that it's invalid, wait 1-2 minutes before using it):\n\n" + inviteLink
	}

	_, err = bot.telebot.Send(sender, msg)
	if err != nil {
		bot.logger.WithError(err).Warn("can't send message on help from web")
	}
}

// on Help command replies with a small help message following two buttons,
// "Groups" and "Settings".
//
// It replies also for /start command. When that command is followed by an UUID,
// then calls startFromUUID to handle the UUID and chat mapping.
func (bot *telegramBot) onHelp(ctx tb.Context, settings chatSettings) {
	bot.botCommandsRequestsTotal.WithLabelValues("start").Inc()

	if err := ctx.Delete(); err != nil {
		bot.logger.WithError(err).Warn("Failed to delete message")
	}

	msg := ctx.Message()
	if msg == nil {
		bot.logger.WithField("updateid", ctx.Update().ID).Warn("Update with nil on Message, ignored")
		return
	}

	if msg.Private() {
		// If a UUID is specified, then we need to lookup for the invite link
		// and send it to the user.
		payload := strings.TrimSpace(msg.Text)
		if strings.ContainsRune(payload, ' ') {
			parts := strings.Split(msg.Text, " ")
			bot.startFromUUID(parts[1], msg.Sender)
			return
		}

		// "Groups" button.
		var buttons [][]tb.InlineButton
		groupsBt := tb.InlineButton{
			Unique: "bt_action_groups",
			Text:   "🇬🇧 Groups / 🇮🇹 Gruppi",
		}
		bot.telebot.Handle(&groupsBt, func(ctx tb.Context) error {
			cb := ctx.Callback()
			if cb == nil {
				bot.logger.WithField("updateid", ctx.Update().ID).Warn("Update with nil on Callback, ignored")
				return nil
			}
			_ = bot.telebot.Respond(ctx.Callback())

			// Note that the second parameter is always ignored in onGroups, so
			// we can avoid a DB lookup.
			bot.sendGroupListForLinks(ctx.Sender(), ctx.Message(), ctx.Message().Chat, nil)
			return nil
		})
		buttons = append(buttons, []tb.InlineButton{groupsBt})

		sender := ctx.Sender()
		if sender == nil {
			bot.logger.WithField("updateid", ctx.Update().ID).Warn("Update with nil on Sender, ignored")
			return
		}
		isGlobalAdmin, err := bot.db.IsGlobalAdmin(sender.ID)
		if err != nil {
			bot.logger.WithError(err).Error("Failed to check if the user is a global admin")
			return
		}

		// Settings button.
		// Check if the user is an admin in at least one chat.
		settingsVisible := false
		if !isGlobalAdmin {
			chatrooms, err := bot.db.ListMyChatrooms()
			if err != nil {
				bot.logger.WithError(err).Error("Failed to get chatroom list")
				return
			}
			for _, x := range chatrooms {
				chatsettings, err := bot.getChatSettings(x)
				if err != nil {
					bot.logger.WithError(err).WithField("chatid", x.ID).Warn("Failed to get chatroom settings")
					continue
				}
				if chatsettings.ChatAdmins.IsAdmin(sender) {
					settingsVisible = true
					break
				}
			}
		} else {
			// The global bot admin is always able to see group settings button.
			settingsVisible = true
		}

		if settingsVisible {
			settingsBt := tb.InlineButton{
				Unique: "bt_action_settings",
				Text:   "🇬🇧 Settings / 🇮🇹 Impostazioni",
			}
			bot.telebot.Handle(&settingsBt, func(ctx tb.Context) error {
				_ = bot.telebot.Respond(ctx.Callback())
				bot.sendGroupListForSettings(ctx.Sender(), ctx.Message(), ctx.Message().Chat, 0)
				return nil
			})
			buttons = append(buttons, []tb.InlineButton{settingsBt})
		}

		// Close button.
		bt := tb.InlineButton{
			Unique: "help_close",
			Text:   "Close / Chiudi",
		}
		buttons = append(buttons, []tb.InlineButton{bt})
		bot.telebot.Handle(&bt, func(ctx tb.Context) error {
			_ = bot.telebot.Respond(ctx.Callback())
			_ = bot.telebot.Delete(ctx.Callback().Message)
			return nil
		})

		// Send reply with buttons.
		err = ctx.Send("🇮🇹 Ciao! Cosa cerchi?\n\n🇬🇧 Hi! What are you looking for?",
			&tb.ReplyMarkup{InlineKeyboard: buttons})
		if err != nil {
			bot.logger.WithError(err).Error("Failed to send message on help")
		}
	}
}
