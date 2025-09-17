package admins

import (
	"log"
	"saxbot/database"
	"saxbot/environment"
	"saxbot/redis"
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
func BanUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember) {
	existingData, err := redis.GetUser(user.User.ID)
	if err != nil {
		existingData = &redis.UserData{
			Username: user.User.Username,
			IsAdmin:  false,
			Warns:    0,
			IsWinner: false,
		}
	}

	existingData.Status = "banned"

	database.SetUserSync(user.User.ID, existingData)
	bot.Ban(chat, user)
}

// Замутить юзера на 30 минут
func MuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember) {
	existingData, err := redis.GetUser(user.User.ID)
	if err != nil {
		existingData = &redis.UserData{
			Username: user.User.Username,
			IsAdmin:  false,
			Warns:    0,
			IsWinner: false,
		}
	}

	existingData.Status = "muted"
	database.SetUserSync(user.User.ID, existingData)

	user.Rights = tele.Rights{CanSendMessages: false}
	bot.Restrict(chat, user)

	go func(userID int64) {
		time.Sleep(30 * time.Minute)

		userData, err := redis.GetUser(userID)
		if err == nil {
			userData.Status = "active"
			database.SetUserSync(userID, userData)
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
func UnmuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember) {
	userData, err := database.GetUserSync(user.User.ID)
	if err != nil {
		log.Printf("UnmuteUser: error for user %d: %v - skipping operation", user.User.ID, err)
		return
	}

	userData.Status = "active"
	if err := database.SetUserSync(user.User.ID, userData); err != nil {
		log.Printf("UnmuteUser: Failed to save persistent data for user %d: %v", user.User.ID, err)
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

	existingData, err := database.GetUserSync(user.User.ID)
	if err != nil {
		log.Printf("SetPref: Redis error for user %d: %v - skipping operation", user.User.ID, err)
		return
	}

	existingData.IsWinner = true
	if err := database.SetUserSync(user.User.ID, existingData); err != nil {
		log.Printf("SetPref: Failed to save persistent data for user %d: %v", user.User.ID, err)
		return
	}
	log.Printf("SetPref: Updated user data in Redis")

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
			promoteParams := map[string]interface{}{
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
			} else {
				log.Printf("SetPref: User promoted successfully with Raw API")
				// Небольшая пауза для обновления статуса в Telegram
				time.Sleep(3 * time.Second)

				// Проверяем, действительно ли пользователь стал админом
				updatedMember, checkErr := bot.ChatMemberOf(chat, user.User)
				if checkErr != nil {
					log.Printf("SetPref: Error checking updated member status: %v", checkErr)
				} else {
					log.Printf("SetPref: Updated member role after promote: %s", updatedMember.Role)
				}
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

	database.SetUserSync(user.User.ID, existingData)
	log.Printf("SetPref: Final Redis update completed")
}

// Удалить преф у пользователя
// TODO: Не работает корректно все, кроме замены статуса в БД, нужно переписать логику
func RemovePref(bot *tele.Bot, chat *tele.Chat) {
	allUsers, err := database.GetAllUsersSync()
	if err != nil {
		log.Printf("RemovePref: Error getting all users: %v", err)
		return
	}

	for userID, userData := range allUsers {
		if !userData.IsWinner {
			continue
		}

		userData.IsWinner = false
		log.Printf("RemovePref: Resetting winner status for user %d", userID)

		user := &tele.User{ID: userID}
		member, err := bot.ChatMemberOf(chat, user)
		if err != nil || member.Role != tele.Administrator {
			database.SetUserSync(userID, userData)
			log.Printf("RemovePref: User %d is not admin, updating data in Redis", userID)
			continue
		}

		if userData.AdminPref != "" {
			log.Printf("RemovePref: User %d has saved admin title, setting it back", userID)
			err = bot.SetAdminTitle(chat, user, userData.AdminPref)
			if err != nil {
				log.Printf("RemovePref: Error setting admin title back: %v", err)
			}
			userData.AdminPref = ""
		} else {
			log.Printf("RemovePref: User %d was not admin before quiz, demoting completely", userID)
			demoteParams := map[string]any{
				"chat_id":                chat.ID,
				"user_id":                userID,
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
				log.Printf("RemovePref: Error demoting user %d with Raw API: %v", userID, demoteErr)
				err = bot.SetAdminTitle(chat, user, "")
				if err != nil {
					log.Printf("RemovePref: Error setting admin title to empty: %v", err)
				}
			} else {
				log.Printf("RemovePref: User %d demoted successfully", userID)
			}
		}

		database.SetUserSync(userID, userData)
	}
}
