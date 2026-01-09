package admins

import (
	"fmt"
	"log"
	"saxbot/database"
	"time"

	tele "gopkg.in/telebot.v4"
)

// Забанить юзера
func BanUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, db *database.PostgresRepository) {
	existingData, err := db.GetUser(user.User.ID)
	if err != nil {
		existingData = database.User{
			UserID:   user.User.ID,
			Username: user.User.Username,
			Warns:    0,
			Status:   "banned",
		}
	}

	existingData.Status = "banned"
	db.SaveUser(&existingData)
	bot.Ban(chat, user)
}

// Разбанить юзера
func UnbanUser(bot *tele.Bot, chat *tele.Chat, user *tele.User, db *database.PostgresRepository) {
	existingData, err := db.GetUser(user.ID)
	if err != nil {
		existingData = database.User{
			UserID:   user.ID,
			Username: user.Username,
			Warns:    0,
			Status:   "active",
		}
	}
	existingData.Status = "active"
	db.SaveUser(&existingData)
	bot.Unban(chat, user)
}

// Замутить юзера на x минут
func MuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, db *database.PostgresRepository, x uint) {
	existingData, err := db.GetUser(user.User.ID)
	if err != nil {
		existingData = database.User{
			UserID:   user.User.ID,
			Username: user.User.Username,
			Warns:    0,
			Status:   "muted",
		}
	}

	existingData.Status = "muted"
	db.SaveUser(&existingData)

	user.Rights = tele.Rights{CanSendMessages: false}
	bot.Restrict(chat, user)

	err = db.SaveUserMutedUntil(existingData.UserID, x)
	if err != nil {
		log.Printf("failed to save muted until time for user %d: %v\ngoing old way with goroutine", existingData.UserID, err)

		// Сохраняем копии переменных для использования в горутине
		userID := user.User.ID
		userCopy := user.User

		go func() {
			time.Sleep(time.Duration(x) * time.Minute)

			userData, err := db.GetUser(userID)
			if err != nil {
				log.Printf("failed to get user %d in unmute goroutine: %v", userID, err)
				return
			}
			if userData.Status != "muted" {
				return
			}

			userData.Status = "active"
			if err := db.SaveUser(&userData); err != nil {
				log.Printf("failed to save active status for user %d in unmute goroutine: %v", userID, err)
			}

			unmuteUser := &tele.ChatMember{
				User: userCopy,
				Rights: tele.Rights{
					CanSendMessages:  true,
					CanSendMedia:     true,
					CanSendAudios:    true,
					CanSendVideos:    true,
					CanSendPhotos:    true,
					CanSendDocuments: true,
					CanSendOther:     true,
				},
			}
			if err := bot.Restrict(chat, unmuteUser); err != nil {
				log.Printf("failed to unrestrict user %d in unmute goroutine: %v", userID, err)
			}
		}()
	}
}

// Размутить юзера досрочно
func UnmuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, db *database.PostgresRepository) {
	userData, err := db.GetUser(user.User.ID)
	if err != nil {
		log.Printf("UnmuteUser: error for user %d: %v - skipping operation", user.User.ID, err)
		return
	}

	userData.Status = "active"
	if err := db.SaveUser(&userData); err != nil {
		log.Printf("UnmuteUser: Failed to save data for user %d: %v", user.User.ID, err)
	}
	user.Rights = tele.Rights{
		CanSendMessages:  true,
		CanSendMedia:     true,
		CanSendAudios:    true,
		CanSendVideos:    true,
		CanSendPhotos:    true,
		CanSendDocuments: true,
		CanSendOther:     true,
	}
	bot.Restrict(chat, user)
}

// Установить админский преф с минимальными правами
func SetPref(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, pref string) {
	log.Printf("SetPref: Starting for user %d (%s) with pref '%s'", user.User.ID, user.User.Username, pref)

	member, err := bot.ChatMemberOf(chat, user.User)
	if err != nil {
		log.Printf("SetPref: Error getting chat member info: %v", err)
	} else {
		log.Printf("SetPref: Current member role: %s, title: '%s'", member.Role, member.Title)
		if member.Role != tele.Administrator {
			botMember, botErr := bot.ChatMemberOf(chat, &tele.User{ID: bot.Me.ID})
			if botErr != nil {
				log.Printf("SetPref: Error getting bot member info: %v", botErr)
			} else {
				log.Printf("SetPref: Bot role: %s, can_promote_members: %v", botMember.Role, botMember.Rights.CanPromoteMembers)
			}

			log.Printf("SetPref: User is not admin, promoting...")
			promoteParams := map[string]any{
				"chat_id":                chat.ID,
				"user_id":                user.User.ID,
				"is_anonymous":           false,
				"can_manage_chat":        false,
				"can_delete_messages":    false,
				"can_manage_video_chats": false,
				"can_restrict_members":   false,
				"can_promote_members":    false,
				"can_change_info":        false,
				"can_invite_users":       true, // Требуется дать одно минимальное право, чтобы ТГ считал пользователя админом
				"can_pin_messages":       false,
				"can_manage_topics":      false,
			}

			_, promoteErr := bot.Raw("promoteChatMember", promoteParams)
			if promoteErr != nil {
				log.Printf("SetPref: Error promoting user with Raw API: %v", promoteErr)
			}

			log.Printf("SetPref: Setting admin title to '%s'", pref)
			err = bot.SetAdminTitle(chat, user.User, pref)
			if err != nil {
				log.Printf("SetPref: Error setting admin title: %v", err)
			} else {
				log.Printf("SetPref: Admin title set successfully")
			}
		}
	}
}

// Удалить преф у пользователя
// На мгновение рестриктит пользователя. Это позволяет лишить его статуса админа
func RemovePref(bot *tele.Bot, chat *tele.Chat, db *database.PostgresRepository) error {
	lastQuiz, err := db.GetLastCompletedQuiz()
	if err != nil {
		return fmt.Errorf("failed to get last quiz: %w", err)
	}
	winnerID := lastQuiz.WinnerID
	user := &tele.User{ID: winnerID}
	member, err := bot.ChatMemberOf(chat, user)
	if err != nil {
		return fmt.Errorf("failed to get chat member from user %d: %w", winnerID, err)
	}
	if member.Role != tele.Administrator {
		log.Printf("RemovePref: User %d is not admin already", winnerID)
		return nil
	}
	member.Rights = tele.Rights{
		CanSendMessages:  true,
		CanSendMedia:     true,
		CanSendAudios:    true,
		CanSendVideos:    true,
		CanSendPhotos:    true,
		CanSendDocuments: false,
		CanSendOther:     false,
	}
	err = bot.Restrict(chat, member)
	if err != nil {
		return fmt.Errorf("failed to temporary restrict user %d: %w", winnerID, err)
	}
	time.Sleep(500 * time.Millisecond)
	member.Rights = tele.Rights{
		CanSendMessages:  true,
		CanSendMedia:     true,
		CanSendAudios:    true,
		CanSendVideos:    true,
		CanSendPhotos:    true,
		CanSendDocuments: true,
		CanSendOther:     true,
	}
	err = bot.Restrict(chat, member)
	if err != nil {
		return fmt.Errorf("failed to unrestrict user %d: %w", winnerID, err)
	}
	return nil
}

// Забрать все права, кроме обычных сообщений
func RestrictUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, db *database.PostgresRepository) error {
	userData, err := db.GetUser(user.User.ID)
	if err != nil {
		return fmt.Errorf("failed to get user %d: %w", user.User.ID, err)
	}
	userData.Status = "restricted"
	db.SaveUser(&userData)
	user.Rights = tele.Rights{
		CanSendMessages:  true,
		CanSendMedia:     false,
		CanSendAudios:    false,
		CanSendVideos:    false,
		CanSendPhotos:    false,
		CanSendDocuments: false,
		CanSendOther:     false,
	}
	err = bot.Restrict(chat, user)
	if err != nil {
		return fmt.Errorf("failed to restrict user %d: %w", user.User.ID, err)
	}
	return nil
}

// Кикнуть юзера без бана
func KickUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember) error {
	err := bot.Ban(chat, user)
	if err != nil {
		return fmt.Errorf("failed to temporary ban user %d: %w", user.User.ID, err)
	}
	time.Sleep(time.Second)
	err = bot.Unban(chat, user.User)
	if err != nil {
		return fmt.Errorf("failed to unban kicked user %d: %w", user.User.ID, err)
	}
	return nil
}

// Размутить юзеров по таймеру
func UnmuteUsersByTime(bot *tele.Bot, chat *tele.Chat, db *database.PostgresRepository) {
	users, err := db.GetAllMutedToUnmute()
	if err != nil {
		log.Printf("failed to get users to unmute: %v", err)
	}
	channels, err := db.GetAllMutedChannelsToUnmute()
	if err != nil {
		log.Printf("failed to get channels to unmute: %v", err)
	}
	if len(users) == 0 && len(channels) == 0 {
		log.Println("got no users to unmute")
	}
	for _, user := range users {
		chatMember, err := bot.ChatMemberOf(chat, &tele.User{ID: user.UserID})
		if err != nil {
			log.Printf("failed to get chat member of user %d: %v", user.UserID, err)
			continue
		}
		UnmuteUser(bot, chat, chatMember, db)
	}
	for _, channel := range channels {
		channel.Status = "active"
		db.SaveChannel(&channel)
	}
}
