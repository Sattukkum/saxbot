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

	go func(userID int64) {
		time.Sleep(time.Duration(x) * time.Minute)

		userData, err := db.GetUser(userID)
		if userData.Status != "muted" {
			return
		}
		if err == nil {
			userData.Status = "active"
			db.SaveUser(&userData)
		}

		unmuteUser := &tele.ChatMember{
			User: user.User,
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
		bot.Restrict(chat, unmuteUser)
	}(user.User.ID)
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
		return fmt.Errorf("failed to get last quiz: %v", err)
	}
	winnerID := lastQuiz.WinnerID
	user := &tele.User{ID: winnerID}
	member, err := bot.ChatMemberOf(chat, user)
	if err != nil {
		return fmt.Errorf("failed to get chat member from user %d: %v", winnerID, err)
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
		return fmt.Errorf("failed to temporary restrict user %d: %v", winnerID, err)
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
		return fmt.Errorf("failed to unrestrict user %d: %v", winnerID, err)
	}
	return nil
}

// Забрать все права, кроме обычных сообщений
func RestrictUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, db *database.PostgresRepository) error {
	userData, err := db.GetUser(user.User.ID)
	if err != nil {
		return fmt.Errorf("failed to get user %d: %v", user.User.ID, err)
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
		return fmt.Errorf("failed to restrict user %d: %v", user.User.ID, err)
	}
	return nil
}

func KickUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember) error {
	err := bot.Ban(chat, user)
	if err != nil {
		return fmt.Errorf("failed to temporary ban user %d: %v", user.User.ID, err)
	}
	time.Sleep(time.Second)
	err = bot.Unban(chat, user.User)
	if err != nil {
		return fmt.Errorf("failed to unban kicked user %d: %v", user.User.ID, err)
	}
	return nil
}
