package messages

import (
	"log"

	tele "gopkg.in/telebot.v4"
)

func SendMessage(c tele.Context, text string, threadID int) error {
	if threadID != 0 {
		log.Printf("Attempting to send message to thread %d: %s", threadID, text)

		// Попробуем несколько вариантов отправки

		// Вариант 1: С ThreadID
		opts := &tele.SendOptions{
			ThreadID: threadID,
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			log.Printf("Method 1 failed (ThreadID): %v", err)

			// Вариант 2: Попробуем ответить на исходное сообщение (если это reply)
			if c.Message() != nil {
				replyOpts := &tele.SendOptions{
					ReplyTo: c.Message(),
				}
				_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
				if err2 == nil {
					log.Printf("Method 2 succeeded (ReplyTo)")
					return nil
				}
				log.Printf("Method 2 failed (ReplyTo): %v", err2)
			}

			// Вариант 3: Обычная отправка без параметров
			log.Printf("Fallback: sending without any special parameters")
			return c.Send(text)
		}
		log.Printf("Method 1 succeeded (ThreadID)")
		return err
	}
	// Обычная отправка
	return c.Send(text)
}

// replyToOriginalMessage отвечает на исходное сообщение (на которое отвечал админ)
func ReplyToOriginalMessage(c tele.Context, text string, threadID int) error {
	if !c.Message().IsReply() {
		// Если это не ответ, используем обычную отправку
		return SendMessage(c, text, threadID)
	}

	originalMessage := c.Message().ReplyTo
	if threadID != 0 {
		log.Printf("Attempting to reply to original message in thread %d: %s", threadID, text)

		// Попробуем несколько вариантов ответа на исходное сообщение

		// Вариант 1: С ThreadID и ReplyTo на исходное сообщение
		opts := &tele.SendOptions{
			ThreadID: threadID,
			ReplyTo:  originalMessage,
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			log.Printf("Original reply method 1 failed (ThreadID+ReplyTo original): %v", err)

			// Вариант 2: Только ReplyTo на исходное сообщение, без ThreadID
			replyOpts := &tele.SendOptions{
				ReplyTo: originalMessage,
			}
			_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
			if err2 == nil {
				log.Printf("Original reply method 2 succeeded (ReplyTo original only)")
				return nil
			}
			log.Printf("Original reply method 2 failed (ReplyTo original only): %v", err2)

			// Вариант 3: Обычная отправка в тред
			log.Printf("Fallback: using sendMessage")
			return SendMessage(c, text, threadID)
		}
		log.Printf("Original reply method 1 succeeded (ThreadID+ReplyTo original)")
		return err
	}
	// Обычный ответ на исходное сообщение
	replyOpts := &tele.SendOptions{
		ReplyTo: originalMessage,
	}
	_, err := c.Bot().Send(c.Chat(), text, replyOpts)
	return err
}

// replyMessage отвечает на сообщение с учетом топика (если есть)
func ReplyMessage(c tele.Context, text string, threadID int) error {
	if threadID != 0 {
		log.Printf("Attempting to reply to thread %d: %s", threadID, text)

		// Попробуем несколько вариантов ответа

		// Вариант 1: С ThreadID и ReplyTo
		opts := &tele.SendOptions{
			ThreadID: threadID,
			ReplyTo:  c.Message(),
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			log.Printf("Reply method 1 failed (ThreadID+ReplyTo): %v", err)

			// Вариант 2: Только ReplyTo, без ThreadID
			replyOpts := &tele.SendOptions{
				ReplyTo: c.Message(),
			}
			_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
			if err2 == nil {
				log.Printf("Reply method 2 succeeded (ReplyTo only)")
				return nil
			}
			log.Printf("Reply method 2 failed (ReplyTo only): %v", err2)

			// Вариант 3: Обычный ответ
			log.Printf("Fallback: using standard reply")
			return c.Reply(text)
		}
		log.Printf("Reply method 1 succeeded (ThreadID+ReplyTo)")
		return err
	}
	// Обычный ответ
	return c.Reply(text)
}
