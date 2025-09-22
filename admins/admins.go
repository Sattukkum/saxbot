package admins

import (
	"log"
	"saxbot/domain"
	"saxbot/environment"
	"saxbot/sync"
	"slices"
	"time"

	tele "gopkg.in/telebot.v4"
)

// Если пользователь админ - true
func IsAdmin(userID int64) bool {
	admins := environment.GetAdmins()
	return slices.Contains(admins, userID)
}

// Забанить юзера
func BanUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, sync sync.SyncService) {
	existingData, err := sync.GetUser(user.User.ID)
	if err != nil {
		existingData = domain.User{
			UserID:    user.User.ID,
			Username:  user.User.Username,
			IsAdmin:   false,
			Warns:     0,
			Status:    "banned",
			IsWinner:  false,
			AdminPref: "",
		}
	}

	sync.SaveUser(&existingData)
	bot.Ban(chat, user)
}

// Замутить юзера на 30 минут
func MuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, sync sync.SyncService) {
	existingData, err := sync.GetUser(user.User.ID)
	if err != nil {
		existingData = domain.User{
			UserID:    user.User.ID,
			Username:  user.User.Username,
			IsAdmin:   false,
			Warns:     0,
			Status:    "muted",
			IsWinner:  false,
			AdminPref: "",
		}
	}

	existingData.Status = "muted"
	sync.SaveUser(&existingData)

	user.Rights = tele.Rights{CanSendMessages: false}
	bot.Restrict(chat, user)

	go func(userID int64) {
		time.Sleep(30 * time.Minute)

		userData, err := sync.GetUser(userID)
		if err == nil {
			userData.Status = "active"
			sync.SaveUser(&userData)
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
func UnmuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, sync sync.SyncService) {
	userData, err := sync.GetUser(user.User.ID)
	if err != nil {
		log.Printf("UnmuteUser: error for user %d: %v - skipping operation", user.User.ID, err)
		return
	}

	userData.Status = "active"
	if err := sync.SaveUser(&userData); err != nil {
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
func SetPref(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, pref string, sync sync.SyncService) {
	log.Printf("SetPref: Starting for user %d (%s) with pref '%s'", user.User.ID, user.User.Username, pref)

	existingData, err := sync.GetUser(user.User.ID)
	if err != nil {
		log.Printf("SetPref: Database error for user %d: %v - skipping operation", user.User.ID, err)
		return
	}

	existingData.IsWinner = true
	if err := sync.SaveUser(&existingData); err != nil {
		log.Printf("SetPref: Failed to save persistent data for user %d: %v", user.User.ID, err)
	}

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
		} else {
			if member.Title != "" {
				existingData.AdminPref = member.Title
				log.Printf("SetPref: Saved existing admin title: '%s'", member.Title)
			}
		}
	}

	log.Printf("SetPref: Setting admin title to '%s'", pref)
	err = bot.SetAdminTitle(chat, user.User, pref)
	if err != nil {
		log.Printf("SetPref: Error setting admin title: %v", err)
	} else {
		log.Printf("SetPref: Admin title set successfully")
	}

	sync.SaveUser(&existingData)
}

// Удалить преф у пользователя
// TODO: Не работает корректно все, кроме замены статуса в БД, нужно переписать логику
func RemovePref(bot *tele.Bot, chat *tele.Chat, sync sync.SyncService) {
	allUsers, err := sync.GetAllUsers()
	if err != nil {
		log.Printf("RemovePref: Error getting all users: %v", err)
		return
	}

	for _, userData := range allUsers {
		if !userData.IsWinner {
			continue
		}

		userData.IsWinner = false
		log.Printf("RemovePref: Resetting winner status for user %d", userData.UserID)

		user := &tele.User{ID: userData.UserID}
		member, err := bot.ChatMemberOf(chat, user)
		if err != nil || member.Role != tele.Administrator {
			sync.SaveUser(&userData)
			log.Printf("RemovePref: User %d is not admin, updating data in Redis", userData.UserID)
			continue
		}

		if userData.AdminPref != "" {
			log.Printf("RemovePref: User %d has saved admin title, setting it back", userData.UserID)
			err = bot.SetAdminTitle(chat, user, userData.AdminPref)
			if err != nil {
				log.Printf("RemovePref: Error setting admin title back: %v", err)
			}
			userData.AdminPref = ""
		} else {
			log.Printf("RemovePref: User %d was not admin before quiz, demoting completely", userData.UserID)
			demoteParams := map[string]any{
				"chat_id":                chat.ID,
				"user_id":                userData.UserID,
				"is_anonymous":           false,
				"can_manage_chat":        false,
				"can_delete_messages":    false,
				"can_manage_video_chats": false,
				"can_restrict_members":   false,
				"can_promote_members":    false,
				"can_change_info":        false,
				"can_invite_users":       false,
				"can_pin_messages":       false,
				"can_manage_topics":      false,
			}

			_, demoteErr := bot.Raw("promoteChatMember", demoteParams)
			if demoteErr != nil {
				log.Printf("RemovePref: Error demoting user %d with Raw API: %v", userData.UserID, demoteErr)
				err = bot.SetAdminTitle(chat, user, "")
				if err != nil {
					log.Printf("RemovePref: Error setting admin title to empty: %v", err)
				}
			} else {
				log.Printf("RemovePref: User %d demoted successfully", userData.UserID)
			}
		}

		sync.SaveUser(&userData)
	}
}
