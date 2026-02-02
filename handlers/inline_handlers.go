package handlers

import (
	"fmt"
	"log"
	"saxbot/database"
	"saxbot/messages"
	textcases "saxbot/text_cases"
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
func handleUserMenu(c tele.Context) error {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBirthday := menu.Data("🎂 Указать дату рождения", "set_birthday")
	btnMusic := menu.Data("Послушать или скачать трек", "show_music")
	menu.Inline(menu.Row(btnBirthday), menu.Row(btnMusic))

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
	btnMusic := menu.Data("Послушать или скачать трек", "show_music")
	menu.Inline(menu.Row(btnBirthday), menu.Row(btnMuted), menu.Row(btnRestricted), menu.Row(btnMusic))

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

// handleShowMusicCallback показывает меню выбора альбома (только для админов в ЛС).
func handleShowMusicCallback(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	if err := c.Respond(); err != nil {
		return err
	}
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	albumsMap := textcases.GetAlbums()
	// Кнопки альбомов: album_1 .. album_5
	var rows []tele.Row
	for i := 1; i <= 5; i++ {
		if name, ok := albumsMap[i]; ok {
			btn := menu.Data(name, fmt.Sprintf("album_%d", i))
			rows = append(rows, menu.Row(btn))
		}
	}
	menu.Inline(rows...)
	return c.Send("Выберите альбом:", &tele.SendOptions{ReplyMarkup: menu})
}

// handleAlbumCallback показывает треклист выбранного альбома.
func handleAlbumCallback(c tele.Context, chatMessageHandler *ChatMessageHandler, albumID int) error {
	if err := c.Respond(); err != nil {
		return err
	}
	tracklist := textcases.GetAlbumTracklist(albumID)
	if tracklist == nil {
		return c.Send("Альбом не найден.")
	}
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	var rows []tele.Row
	var row tele.Row
	for i := 1; ; i++ {
		name, ok := tracklist[i]
		if !ok {
			break
		}
		btn := menu.Data(name, fmt.Sprintf("track_%d_%d", albumID, i))
		row = append(row, btn)
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	backBtn := menu.Data("Вернуться в главное меню", "main_menu")
	row = append(row, backBtn)
	if len(row) > 0 {
		rows = append(rows, row)
	}
	menu.Inline(rows...)
	albumName := textcases.GetAlbums()[albumID]
	return c.Send(fmt.Sprintf("Альбом «%s». Выберите трек:", albumName), &tele.SendOptions{ReplyMarkup: menu})
}

// handleTrackCallback отправляет аудио трека с описанием.
func handleTrackCallback(c tele.Context, chatMessageHandler *ChatMessageHandler, albumID, trackID int) error {
	if err := c.Respond(); err != nil {
		return err
	}
	audioData := textcases.GetTrack(albumID, trackID, chatMessageHandler.Rep)
	if audioData.ID == 0 {
		return c.Send("Внутренняя ошибка базы данных. Попробуйте позже. Если ошибка повторяется, обратитесь к администратору.")
	}
	caption := fmt.Sprintf("<b>%s</b>\n\n<b>Комментарий от Ника:</b>\n\n%s\n\nЧтобы скачать трек, нажми на него правой кнопкой мыши и выбери соответствующее действие. Чтобы вернуться в главное меню, напиши команду \"меню\" или кликни на кнопку выше", audioData.Name, audioData.Description)
	if audioData.ClipURL != "" {
		caption = fmt.Sprintf("%s\n\n<b><a href=\"%s\">Смотреть клип</a></b>", caption, audioData.ClipURL)
	}
	audio := &tele.Audio{
		File: tele.File{
			FileID: audioData.FileID,
		},
		Caption: caption,
	}
	opts := &tele.SendOptions{
		ParseMode: tele.ModeHTML,
	}
	_, err := chatMessageHandler.Bot.Send(c.Chat(), audio, opts)
	if err != nil {
		log.Printf("failed to send audio %s: %v", audioData.FileID, err)
		return c.Send("Не удалось отправить трек. Попробуйте позже.")
	}
	return nil
}
