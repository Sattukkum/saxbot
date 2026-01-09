package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Получить канал по ID (создает нового, если не найден)
func (p *PostgresRepository) GetChannel(senderChatID int64) (Channel, error) {
	var channel Channel
	err := p.db.Where("sender_chat_id = ?", senderChatID).First(&channel).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("Creating new channel %d (not found in database)", senderChatID)

			newChannel := Channel{
				SenderChatID: senderChatID,
				Title:        "",
				Warns:        0,
				Status:       "active",
			}

			err = p.db.Create(&newChannel).Error
			if err != nil {
				return Channel{}, fmt.Errorf("failed to create channel: %w", err)
			}

			log.Printf("Created new channel %d", senderChatID)
			return newChannel, nil
		}
		log.Printf("Failed to get channel %d, %v", senderChatID, err)
		return Channel{}, fmt.Errorf("failed to get channel: %w", err)
	}
	return channel, nil
}

// Сохранить канал в базе данных
func (p *PostgresRepository) SaveChannel(channel *Channel) error {
	return p.db.Save(channel).Error
}

// Обновить title канала
func (p *PostgresRepository) UpdateTitle(senderChatID int64, title string) error {
	result := p.db.Model(&Channel{}).Where("sender_chat_id = ?", senderChatID).Update("title", title)
	if result.Error != nil {
		return fmt.Errorf("failed to update title: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("channel with id %d not found", senderChatID)
	}
	return nil
}

// Обновить количество предупреждений канала
func (p *PostgresRepository) UpdateChannelWarns(senderChatID int64, delta int) error {
	channel, err := p.GetChannel(senderChatID)
	if err != nil {
		return err
	}

	channel.Warns += delta
	if channel.Warns < 0 {
		channel.Warns = 0
	}

	return p.SaveChannel(&channel)
}

// Получить время размута канала
func (p *PostgresRepository) GetChannelMutedUntil(senderChatID int64) (time.Time, error) {
	channel, err := p.GetChannel(senderChatID)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to ger channel %d: %w", senderChatID, err)
	}
	return channel.MutedUntil, nil
}

// Сохранить время размута канала
func (p *PostgresRepository) SaveChannelMutedUntil(senderChatID int64, minutes uint) error {
	if minutes == 0 {
		return fmt.Errorf("minutes must be positive, got %d", minutes)
	}
	now := time.Now().In(MoscowTZ)
	mutedUntil := now.Add(time.Duration(minutes) * time.Minute)

	channel, err := p.GetChannel(senderChatID)
	if err != nil {
		return fmt.Errorf("failed to get channel %d: %w", senderChatID, err)
	}

	channel.MutedUntil = mutedUntil
	return p.SaveChannel(&channel)
}

// Получить все каналы, которые пора размутить
func (p *PostgresRepository) GetAllMutedChannelsToUnmute() ([]Channel, error) {
	var channels []Channel
	now := time.Now().In(MoscowTZ)
	query := p.db.Where(
		`status = 'muted'
		AND muted_until IS NOT NULL
		AND muted_until < ?`,
		now,
	)

	err := query.Find(&channels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get channels to unmute: %w", err)
	}

	return channels, nil
}
