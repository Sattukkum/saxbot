package messages

import (
	tele "gopkg.in/telebot.v4"
)

// Отправить сообщение в тред (если есть)
func SendMessage(c tele.Context, text string, threadID int) error {
	if threadID != 0 {
		opts := &tele.SendOptions{
			ThreadID: threadID,
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			return c.Send(text)
		}
	}
	return nil
}

// Ответить на исходное сообщение (на которое отвечал админ)
func ReplyToOriginalMessage(c tele.Context, text string, threadID int) error {
	if !c.Message().IsReply() {
		return SendMessage(c, text, threadID)
	}

	originalMessage := c.Message().ReplyTo
	if threadID != 0 {
		opts := &tele.SendOptions{
			ThreadID: threadID,
			ReplyTo:  originalMessage,
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			replyOpts := &tele.SendOptions{
				ReplyTo: originalMessage,
			}
			_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
			if err2 == nil {
				return nil
			}
			return SendMessage(c, text, threadID)
		}
		return err
	}
	replyOpts := &tele.SendOptions{
		ReplyTo: originalMessage,
	}
	_, err := c.Bot().Send(c.Chat(), text, replyOpts)
	return err
}

// Ответить на сообщение в тред (если есть)
func ReplyMessage(c tele.Context, text string, threadID int) error {
	if threadID != 0 {
		opts := &tele.SendOptions{
			ThreadID: threadID,
			ReplyTo:  c.Message(),
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			replyOpts := &tele.SendOptions{
				ReplyTo: c.Message(),
			}
			_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
			if err2 == nil {
				return nil
			}
			return c.Reply(text)
		}
		return err
	}
	return c.Reply(text)
}
