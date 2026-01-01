package handlers

import (
	"fmt"
	"log"
	"saxbot/activities"
	"saxbot/database"
	"slices"

	tele "gopkg.in/telebot.v4"
)

type ChatMessageHandler struct {
	AllowedChats    []int64
	AdminsList      []int64
	AdminsUsernames []string
	QuizManager     *activities.QuizManager
	Rep             *database.PostgresRepository
	Bot             *tele.Bot
	ChatMessage     *ChatMessage
	UserStates      map[int64]string // Состояния пользователей (userID -> state)
}

type ChatMessage struct {
	isReply         bool
	replyTo         *tele.Message
	sender          *tele.User
	text            string
	chat            *tele.Chat
	threadID        int
	chatAdmin       bool
	replyToAdmin    bool
	replyToAppeal   string
	isWinner        bool
	userData        *database.User
	replyToUserData *database.User
	adminRole       string
	appeal          string
	replyToID       int64
}

// Геттеры для доступа к полям ChatMessage
func (cm *ChatMessage) IsReply() bool {
	if cm == nil {
		return false
	}
	return cm.isReply
}

func (cm *ChatMessage) ReplyTo() *tele.Message {
	if cm == nil {
		return nil
	}
	return cm.replyTo
}

func (cm *ChatMessage) Sender() *tele.User {
	if cm == nil {
		return nil
	}
	return cm.sender
}

func (cm *ChatMessage) Text() string {
	if cm == nil {
		return ""
	}
	return cm.text
}

func (cm *ChatMessage) Chat() *tele.Chat {
	if cm == nil {
		return nil
	}
	return cm.chat
}

func (cm *ChatMessage) ThreadID() int {
	if cm == nil {
		return 0
	}
	return cm.threadID
}

func (cm *ChatMessage) ChatAdmin() bool {
	if cm == nil {
		return false
	}
	return cm.chatAdmin
}

func (cm *ChatMessage) ReplyToAdmin() bool {
	if cm == nil {
		return false
	}
	return cm.replyToAdmin
}

func (cm *ChatMessage) ReplyToAppeal() string {
	if cm == nil {
		return ""
	}
	return cm.replyToAppeal
}

func (cm *ChatMessage) IsWinner() bool {
	if cm == nil {
		return false
	}
	return cm.isWinner
}

func (cm *ChatMessage) UserData() *database.User {
	if cm == nil {
		return nil
	}
	return cm.userData
}

func (cm *ChatMessage) ReplyToUserData() *database.User {
	if cm == nil {
		return nil
	}
	return cm.replyToUserData
}

func (cm *ChatMessage) AdminRole() string {
	if cm == nil {
		return ""
	}
	return cm.adminRole
}

func (cm *ChatMessage) Appeal() string {
	if cm == nil {
		return ""
	}
	return cm.appeal
}

func (cm *ChatMessage) ReplyToID() int64 {
	if cm == nil {
		return 0
	}
	return cm.replyToID
}

func initChatMessage(c tele.Context, handler *ChatMessageHandler) (*ChatMessage, error) {
	// Валидация входных данных
	if c == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is nil")
	}
	if handler.Rep == nil {
		return nil, fmt.Errorf("repository is nil")
	}
	if handler.QuizManager == nil {
		return nil, fmt.Errorf("quiz manager is nil")
	}

	msg := c.Message()
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}
	if msg.Sender == nil {
		return nil, fmt.Errorf("message sender is nil")
	}
	if msg.Chat == nil {
		return nil, fmt.Errorf("message chat is nil")
	}

	chatMsg := &ChatMessage{
		isReply:  msg.IsReply(),
		replyTo:  msg.ReplyTo,
		sender:   msg.Sender,
		text:     msg.Text,
		chat:     msg.Chat,
		threadID: msg.ThreadID,
	}

	// Определяем, является ли отправитель каналом-админом
	var chatAdmin bool
	if msg.SenderChat != nil {
		chatID := msg.SenderChat.ID
		if slices.Contains(handler.AdminsList, chatID) {
			chatAdmin = true
		}
	}
	chatMsg.chatAdmin = chatAdmin

	// Формируем обращение к отправителю
	appeal := "@" + msg.Sender.Username
	if appeal == "@" {
		if msg.Sender.FirstName == "" {
			appeal = fmt.Sprintf("User %d", msg.Sender.ID)
		} else {
			appeal = msg.Sender.FirstName
		}
	}
	chatMsg.appeal = appeal

	// Получаем данные пользователя из БД
	userID := msg.Sender.ID
	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}

	userData, err := handler.Rep.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data for user %d: %w", userID, err)
	}

	// Обновляем username и firstname, если изменились
	if userData.Username != msg.Sender.Username || userData.FirstName != msg.Sender.FirstName {
		userData.Username = msg.Sender.Username
		userData.FirstName = msg.Sender.FirstName
		if err := handler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save persistent username update for user %d: %v", userID, err)
			// Не возвращаем ошибку, так как это не критично
		}
	}
	chatMsg.userData = &userData

	// Обработка ответа на сообщение
	if chatMsg.isReply {
		if msg.ReplyTo == nil {
			return nil, fmt.Errorf("reply flag is set but ReplyTo is nil")
		}
		if msg.ReplyTo.Sender == nil {
			return nil, fmt.Errorf("reply message sender is nil")
		}

		replyToID := msg.ReplyTo.Sender.ID
		if replyToID == 0 {
			return nil, fmt.Errorf("invalid reply to user ID: %d", replyToID)
		}
		chatMsg.replyToID = replyToID

		replyToAppeal := "@" + msg.ReplyTo.Sender.Username
		if replyToAppeal == "@" {
			if msg.ReplyTo.Sender.FirstName == "" {
				replyToAppeal = fmt.Sprintf("User %d", replyToID)
			} else {
				replyToAppeal = msg.ReplyTo.Sender.FirstName
			}
		}
		chatMsg.replyToAppeal = replyToAppeal

		replyToUser, err := handler.Rep.GetUser(replyToID)
		if err != nil {
			return nil, fmt.Errorf("failed to get reply to user data for user %d: %w", replyToID, err)
		}

		if replyToUser.Username != msg.ReplyTo.Sender.Username {
			replyToUser.Username = msg.ReplyTo.Sender.Username
			if err := handler.Rep.SaveUser(&replyToUser); err != nil {
				log.Printf("Failed to save persistent username update for reply user %d: %v", replyToID, err)
				// Не возвращаем ошибку, так как это не критично
			}
		}
		chatMsg.replyToUserData = &replyToUser

		replyToAdmin := handler.Rep.IsAdmin(replyToID)
		chatMsg.replyToAdmin = replyToAdmin
	}

	// Определяем роль админа
	var adminRole = ""
	isAdmin := handler.Rep.IsAdmin(userData.UserID)
	if isAdmin {
		adminRole, err = handler.Rep.GetAdminRole(userData.UserID)
		if err != nil {
			log.Printf("failed to get admin role for user %d, consider it junior: %v", userData.UserID, err)
			adminRole = "junior"
		}
	}
	if chatAdmin {
		adminRole = "senior"
	}
	chatMsg.adminRole = adminRole

	// Проверяем, является ли пользователь победителем квиза
	isWinner := userData.UserID == handler.QuizManager.Winner()
	chatMsg.isWinner = isWinner

	return chatMsg, nil
}

func initPrivateMessage(c tele.Context, handler *ChatMessageHandler) (*ChatMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is nil")
	}
	if handler.Rep == nil {
		return nil, fmt.Errorf("repository is nil")
	}
	if handler.QuizManager == nil {
		return nil, fmt.Errorf("quiz manager is nil")
	}

	msg := c.Message()
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}
	if msg.Sender == nil {
		return nil, fmt.Errorf("message sender is nil")
	}

	chatMsg := &ChatMessage{
		sender: msg.Sender,
		text:   msg.Text,
		appeal: "@" + msg.Sender.Username,
	}

	userID := msg.Sender.ID
	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}

	// Формируем обращение к отправителю
	appeal := "@" + msg.Sender.Username
	if appeal == "@" {
		if msg.Sender.FirstName == "" {
			appeal = fmt.Sprintf("User %d", msg.Sender.ID)
		} else {
			appeal = msg.Sender.FirstName
		}
	}
	chatMsg.appeal = appeal

	userData, err := handler.Rep.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data for user %d: %w", userID, err)
	}

	// Обновляем username и firstname, если изменились
	if userData.Username != msg.Sender.Username || userData.FirstName != msg.Sender.FirstName {
		userData.Username = msg.Sender.Username
		userData.FirstName = msg.Sender.FirstName
		if err := handler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save persistent username update for user %d: %v", userID, err)
			// Не возвращаем ошибку, так как это не критично
		}
	}
	chatMsg.userData = &userData

	var adminRole = ""
	isAdmin := handler.Rep.IsAdmin(userData.UserID)
	if isAdmin {
		adminRole, err = handler.Rep.GetAdminRole(userData.UserID)
		if err != nil {
			log.Printf("failed to get admin role for user %d, consider it junior: %v", userData.UserID, err)
			adminRole = "junior"
		}
	}
	chatMsg.adminRole = adminRole

	// Определяем, является ли пользователь победителем квиза
	isWinner := userData.UserID == handler.QuizManager.Winner()
	chatMsg.isWinner = isWinner

	return chatMsg, nil
}

// GetUserState возвращает текущее состояние пользователя для личной переписки
// Если состояние не установлено, возвращает "default"
func (h *ChatMessageHandler) GetUserState(userID int64) string {
	if h.UserStates == nil {
		return "default"
	}
	state, exists := h.UserStates[userID]
	if !exists {
		return "default"
	}
	return state
}

// SetUserState устанавливает состояние пользователя для личной переписки
func (h *ChatMessageHandler) SetUserState(userID int64, state string) {
	if h.UserStates == nil {
		h.UserStates = make(map[int64]string)
	}
	h.UserStates[userID] = state
}
