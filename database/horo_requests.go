package database

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func (p *PostgresRepository) SetHoroscope(h Horoscope) error {

	var existing Horoscope

	err := p.db.First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p.db.Create(&h).Error
	}

	if err != nil {
		return err
	}

	h.ID = existing.ID

	return p.db.Save(&h).Error
}

func (p *PostgresRepository) GetHoroscope(sign string) (string, error) {

	column, err := signToColumn(sign)
	if err != nil {
		return "", err
	}

	var result struct {
		Value string
	}

	err = p.db.
		Model(&Horoscope{}).
		Select(column + " as value").
		Limit(1).
		Scan(&result).Error

	return result.Value, err
}

func signToColumn(sign string) (string, error) {

	switch strings.ToLower(sign) {

	case "aries", "овен":
		return "aries", nil

	case "taurus", "телец":
		return "taurus", nil

	case "gemini", "близнецы":
		return "gemini", nil

	case "cancer", "рак":
		return "cancer", nil

	case "leo", "лев":
		return "leo", nil

	case "virgo", "дева":
		return "virgo", nil

	case "libra", "весы":
		return "libra", nil

	case "scorpio", "скорпион":
		return "scorpio", nil

	case "sagittarius", "стрелец":
		return "sagittarius", nil

	case "capricorn", "козерог":
		return "capricorn", nil

	case "aquarius", "водолей":
		return "aquarius", nil

	case "pisces", "рыбы":
		return "pisces", nil

	default:
		return "", fmt.Errorf("unknown zodiac sign")
	}
}
