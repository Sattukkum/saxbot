package parser

import (
	"fmt"
	"log"
	"net/http"
	"saxbot/database"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Horoscope struct {
	Sign string
	Text string
}

func ParseLatestPost(url string, db *database.PostgresRepository) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	post := doc.Find(".tgme_widget_message_text").Last()
	if post.Length() == 0 {
		return fmt.Errorf("no post found")
	}

	text := strings.TrimSpace(post.Text())

	if !isHoroscopePost(text) {
		log.Println("last post is not horoscope")
		return nil
	}

	horoscopes := splitHoroscope(text)

	saveHoroscopes(horoscopes, db)

	return nil
}

func saveHoroscopes(horoscopes []Horoscope, db *database.PostgresRepository) {

	var dbHoroscope database.Horoscope

	for _, h := range horoscopes {

		switch h.Sign {

		case "♈️Овен", "Овен":
			dbHoroscope.Aries = h.Text

		case "♉️Телец", "Телец":
			dbHoroscope.Taurus = h.Text

		case "♊️Близнецы", "Близнецы":
			dbHoroscope.Gemini = h.Text

		case "♋️Рак", "Рак":
			dbHoroscope.Cancer = h.Text

		case "♌️Лев", "Лев":
			dbHoroscope.Leo = h.Text

		case "♍️Дева", "Дева":
			dbHoroscope.Virgo = h.Text

		case "♎️Весы", "Весы":
			dbHoroscope.Libra = h.Text

		case "♏️Скорпион", "Скорпион":
			dbHoroscope.Scorpio = h.Text

		case "♐️Стрелец", "Стрелец":
			dbHoroscope.Sagittarius = h.Text

		case "♑️Козерог", "Козерог":
			dbHoroscope.Capricorn = h.Text

		case "♒️Водолей", "Водолей":
			dbHoroscope.Aquarius = h.Text

		case "♓️Рыбы", "Рыбы":
			dbHoroscope.Pisces = h.Text
		}
	}

	err := db.SetHoroscope(dbHoroscope)
	if err != nil {
		log.Println("failed to save horoscope:", err)
	}
}

func isHoroscopePost(text string) bool {
	signs := []string{
		"♈️Овен",
		"♉️Телец",
		"♊️Близнецы",
		"♋️Рак",
	}

	count := 0

	for _, s := range signs {
		if strings.Contains(text, s) {
			count++
		}
	}

	return count >= 3
}

func splitHoroscope(text string) []Horoscope {

	signs := []string{
		"♈️Овен:",
		"♉️Телец:",
		"♊️Близнецы:",
		"♋️Рак:",
		"♌️Лев:",
		"♍️Дева:",
		"♎️Весы:",
		"♏️Скорпион:",
		"♐️Стрелец:",
		"♑️Козерог:",
		"♒️Водолей:",
		"♓️Рыбы:",
	}

	var result []Horoscope

	for i, sign := range signs {

		start := strings.Index(text, sign)
		if start == -1 {
			continue
		}

		start += len(sign)

		end := len(text)

		for j := i + 1; j < len(signs); j++ {
			next := strings.Index(text, signs[j])
			if next != -1 {
				end = next
				break
			}
		}

		segment := strings.TrimSpace(text[start:end])

		result = append(result, Horoscope{
			Sign: strings.TrimSuffix(sign, ":"),
			Text: segment,
		})
	}

	return result
}
