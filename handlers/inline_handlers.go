package handlers

import (
	"fmt"
	"saxbot/database"
	"saxbot/messages"
	"time"

	tele "gopkg.in/telebot.v4"
)

func handleSaveBirthday(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	userData := chatMsg.UserData()
	if userData == nil {
		return fmt.Errorf("user data is nil")
	}
	// Сбрасываем состояние пользователя после сохранения даты рождения
	chatMessageHandler.SetUserState(userData.UserID, "default")
	birthday := chatMsg.Text()
	if birthday == "" {
		return messages.ReplyMessage(c, "Введите дату рождения в формате DD.MM.YYYY", chatMsg.ThreadID())
	}
	birthdayTime, err := time.Parse("02.01.2006", birthday)
	if err != nil {
		return messages.ReplyMessage(c, "Неверный формат даты. Пожалуйста, используйте DD.MM.YYYY", chatMsg.ThreadID())
	}
	if err := chatMessageHandler.Rep.UpdateUserBirthday(userData.UserID, birthdayTime); err != nil {
		return messages.ReplyMessage(c, "Не удалось сохранить дату рождения", chatMsg.ThreadID())
	}
	return messages.ReplyMessage(c, fmt.Sprintf("Дата рождения %s сохранена", birthdayTime.Format("02.01.2006")), chatMsg.ThreadID())
}

// handleShowBirthdayMenu показывает инлайн-меню с кнопкой для указания даты рождения
// Доступно только в личных сообщениях
func handleShowBirthdayMenu(c tele.Context) error {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBirthday := menu.Data("🎂 Указать дату рождения", "set_birthday")
	menu.Inline(menu.Row(btnBirthday))

	text := "Выберите действие:"
	return c.Reply(text, &tele.SendOptions{ReplyMarkup: menu})
}

// handleBirthdayCallback обрабатывает нажатие на кнопку "Указать дату рождения"
func handleBirthdayCallback(c tele.Context) error {
	// Отвечаем на callback, чтобы убрать индикатор загрузки
	if err := c.Respond(); err != nil {
		return err
	}

	// Просим пользователя ввести дату рождения
	text := "Пожалуйста, введите дату рождения в формате DD.MM.YYYY (например, 15.03.1990)"
	return c.Send(text)
}

func handleAdminMenu(c tele.Context) error {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBirthday := menu.Data("🎂 Указать дату рождения", "set_birthday")
	btnMuted := menu.Data("Пользователи в муте", "show_muted")
	btnRestricted := menu.Data("Рестриктнутые пользователи", "show_restricted")
	// btnBanned := menu.Data("Забаненные пользователи", "show_banned")
	menu.Inline(menu.Row(btnBirthday), menu.Row(btnMuted), menu.Row(btnRestricted))

	text := "Доступные админ-команды:\nРазмут [id] - размутить пользоваться\nКвиз - информация о сегодняшнем квизе\nВыберите действие:"
	return c.Reply(text, &tele.SendOptions{ReplyMarkup: menu})
}

func handleMutedCallback(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	if err := c.Respond(); err != nil {
		return err
	}
	users, err := chatMessageHandler.Rep.GetAllMutedUsers()
	if err != nil {
		return c.Send("Произошла внутренняя ошибка базы данных. Попробуйте ещё раз")
	}
	if len(users) == 0 {
		return c.Send("В базе данных сейчас нет пользователей в муте")
	} else {
		text := "Вот список пользователей в муте. Пользователя можно размутить досрочно командой \"Размут [id]\":\n"
		for count, user := range users {
			mutedUntilStr := "не установлено"
			if !user.MutedUntil.IsZero() {
				mutedUntilStr = user.MutedUntil.In(database.MoscowTZ).Format("2006-01-02 15:04:05")
			}
			text = text + fmt.Sprintf("%d. @%s, имя: %s, id: %d, время размута %s\n", count+1, user.Username, user.FirstName, user.UserID, mutedUntilStr)
		}
		return c.Send(text)
	}
}

func handleRestrictedCallback(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	if err := c.Respond(); err != nil {
		return err
	}
	users, err := chatMessageHandler.Rep.GetAllRestrictedUsers()
	if err != nil {
		return c.Send("Произошла внутренняя ошибка базы данных. Попробуйте ещё раз")
	}
	if len(users) == 0 {
		return c.Send("В базе данных сейчас нет рестриктнутых пользователей")
	} else {
		text := "Вот список рестриктнутых пользователей. С пользователя можно снять ограничения командой \"Размут [id]\":\n"
		for count, user := range users {
			text = text + fmt.Sprintf("%d. @%s, имя: %s, id: %d\n", count+1, user.Username, user.FirstName, user.UserID)
		}
		return c.Send(text)
	}
}
