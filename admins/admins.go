package admins

import (
	"log"
	"saxbot/environment"
	"saxbot/redis"
	"slices"
	"time"

	tele "gopkg.in/telebot.v4"
)

func IsAdmin(userID int64) bool {
	admins := environment.GetAdmins()
	return slices.Contains(admins, userID)
}

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

	redis.SetUser(user.User.ID, existingData)
	redis.SetUserPersistent(user.User.ID, existingData)
	bot.Ban(chat, user)
}

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
	redis.SetUser(user.User.ID, existingData)
	redis.SetUserPersistent(user.User.ID, existingData)

	user.Rights = tele.Rights{CanSendMessages: false}
	bot.Restrict(chat, user)

	go func(userID int64) {
		time.Sleep(30 * time.Minute)

		userData, err := redis.GetUser(userID)
		if err == nil {
			userData.Status = "active"
			redis.SetUser(userID, userData)
			redis.SetUserPersistent(userID, userData)
		}

		unmuteUser := &tele.ChatMember{
			User:   user.User,
			Rights: tele.Rights{CanSendMessages: true},
		}
		bot.Restrict(chat, unmuteUser)
	}(user.User.ID)
}

func UnmuteUser(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember) {
	existingData, err := redis.GetUser(user.User.ID)
	if err != nil {
		existingData = &redis.UserData{
			Username: user.User.Username,
			IsAdmin:  false,
			Warns:    0,
			IsWinner: false,
		}
	}

	existingData.Status = "active"
	redis.SetUser(user.User.ID, existingData)
	redis.SetUserPersistent(user.User.ID, existingData)
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

func SetPref(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, pref string) {
	log.Printf("SetPref: Starting for user %d (%s) with pref '%s'", user.User.ID, user.User.Username, pref)

	existingData, err := redis.GetUser(user.User.ID)
	if err != nil {
		log.Printf("SetPref: User not found in Redis, creating new data")
		existingData = &redis.UserData{
			Username: user.User.Username,
			IsAdmin:  false,
			Warns:    0,
			IsWinner: false,
		}
	} else {
		log.Printf("SetPref: Found existing user data: IsAdmin=%v, IsWinner=%v", existingData.IsAdmin, existingData.IsWinner)
	}

	existingData.IsWinner = true
	redis.SetUser(user.User.ID, existingData)
	redis.SetUserPersistent(user.User.ID, existingData)
	log.Printf("SetPref: Updated user data in Redis")

	// Получаем текущие права пользователя
	member, err := bot.ChatMemberOf(chat, user.User)
	if err != nil {
		log.Printf("SetPref: Error getting chat member info: %v", err)
		// При ошибке получения информации о пользователе, не промоутим
		// Просто устанавливаем титул без изменения прав
	} else {
		log.Printf("SetPref: Current member role: %s, title: '%s'", member.Role, member.Title)
		if member.Role != tele.Administrator {
			// Сначала проверим права бота
			botMember, botErr := bot.ChatMemberOf(chat, &tele.User{ID: bot.Me.ID})
			if botErr != nil {
				log.Printf("SetPref: Error getting bot member info: %v", botErr)
			} else {
				log.Printf("SetPref: Bot role: %s, can_promote_members: %v", botMember.Role, botMember.Rights.CanPromoteMembers)
			}

			// Если пользователь не админ, промоутим с минимальными правами
			log.Printf("SetPref: User is not admin, promoting...")

			// Попробуем прямой вызов PromoteChatMember
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
				"can_invite_users":       true, // Даем минимальное право
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
			// Если пользователь уже админ, сохраняем его текущий титул
			if member.Title != "" {
				existingData.AdminPref = member.Title
				log.Printf("SetPref: Saved existing admin title: '%s'", member.Title)
			}
		}
	}

	// Устанавливаем только титул, не меняя права
	log.Printf("SetPref: Setting admin title to '%s'", pref)
	err = bot.SetAdminTitle(chat, user.User, pref)
	if err != nil {
		log.Printf("SetPref: Error setting admin title: %v", err)
	} else {
		log.Printf("SetPref: Admin title set successfully")
	}

	// Обновляем данные в Redis после всех изменений
	redis.SetUser(user.User.ID, existingData)
	redis.SetUserPersistent(user.User.ID, existingData)
	log.Printf("SetPref: Final Redis update completed")
}

func RemovePref(bot *tele.Bot, chat *tele.Chat) {
	// Получаем всех пользователей из Redis
	allUsers, err := redis.GetAllUsers()
	if err != nil {
		log.Printf("RemovePref: Error getting all users: %v", err)
		return // Если не удалось получить пользователей, ничего не делаем
	}

	// Проходим по всем пользователям и обрабатываем победителей
	for userID, userData := range allUsers {
		if !userData.IsWinner {
			continue // Пропускаем не-победителей
		}

		// Сбрасываем статус победителя
		userData.IsWinner = false
		log.Printf("RemovePref: Resetting winner status for user %d", userID)

		// Получаем текущие права пользователя в чате
		user := &tele.User{ID: userID}
		member, err := bot.ChatMemberOf(chat, user)
		if err != nil || member.Role != tele.Administrator {
			// Если пользователь не админ, просто обновляем данные в Redis
			redis.SetUser(userID, userData)
			redis.SetUserPersistent(userID, userData)
			log.Printf("RemovePref: User %d is not admin, updating data in Redis", userID)
			continue
		}

		// Если у пользователя был сохранен предыдущий титул, восстанавливаем его
		if userData.AdminPref != "" {
			log.Printf("RemovePref: User %d has saved admin title, setting it back", userID)
			err = bot.SetAdminTitle(chat, user, userData.AdminPref)
			if err != nil {
				log.Printf("RemovePref: Error setting admin title back: %v", err)
			}
			userData.AdminPref = "" // Очищаем сохраненный титул
		} else {
			// Если предыдущего титула не было, значит пользователь стал админом только для квиза
			// Разжаловываем его полностью
			log.Printf("RemovePref: User %d was not admin before quiz, demoting completely", userID)

			// Попробуем разжаловать пользователя через Raw API
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
				// Если не удалось разжаловать, хотя бы уберем титул
				err = bot.SetAdminTitle(chat, user, "")
				if err != nil {
					log.Printf("RemovePref: Error setting admin title to empty: %v", err)
				}
			} else {
				log.Printf("RemovePref: User %d demoted successfully", userID)
			}
		}

		// Обновляем данные в Redis
		redis.SetUser(userID, userData)
		redis.SetUserPersistent(userID, userData)
	}
}
