package textcases

import (
	"log"
	"saxbot/database"
)

var albums = map[int]string{
	1: "Синглы",
	2: "USSR Mixtape",
	3: "KPSS PUNK",
	4: "ЧЕРТОВЩИНА",
	5: "НЕЖИТЬ, ХОЙ",
}

// GetAlbums возвращает карту id альбома -> название (для меню выбора альбома).
func GetAlbums() map[int]string {
	return albums
}

// GetAlbumTracklist возвращает треклист альбома: номер трека -> название.
func GetAlbumTracklist(album int) map[int]string {
	switch album {
	case 1:
		return singlesTracklist
	case 2:
		return ussrTracklist
	case 3:
		return kpssPunkTracklist
	case 4:
		return chertovshinaTracklist
	case 5:
		return nezhitHoiTracklist
	default:
		return nil
	}
}

var chertovshinaTracklist = map[int]string{
	1:  "Быличка",
	2:  "Рейв на Могилке",
	3:  "Охота на Буржуя",
	4:  "Ядерная Зима",
	5:  "Мара",
	6:  "Волк Октября",
	7:  "У нас в Советах",
	8:  "Чертовщина",
	9:  "Красный Кулак",
	10: "Темнота",
}

var kpssPunkTracklist = map[int]string{
	1:  "Village Boy",
	2:  "Moscow",
	3:  "Not Okay",
	4:  "Домой",
	5:  "Ведьма",
	6:  "Gas",
	7:  "Русский Бунт",
	8:  "KPSS PUNK",
	9:  "Огонёк",
	10: "Goodbye America",
}

var nezhitHoiTracklist = map[int]string{
	1:  "Нежить, Хой!",
	2:  "Болотница",
	3:  "Последний Диктатор",
	4:  "Казнить, Казнить, Казнить",
	5:  "Матушка Больна",
	6:  "Автомат не Виноват",
	7:  "Каникулы в КНДР",
	8:  "Серп и Молот",
	9:  "Расправа над Бабой-Ягой",
	10: "Дикая Река",
}

var ussrTracklist = map[int]string{
	1: "USSR KID",
	2: "Russian cyberpunk rave",
	3: "ZAVOD",
	4: "Ghost of Communism",
	5: "Tsar",
	6: "No Money Be Happy",
	7: "Rocket Man",
	8: "Drive On",
}

var singlesTracklist = map[int]string{
	1: "Русская Печаль",
	2: "В объятиях Лешего",
	3: "Белла, Чао!",
	4: "Ночь перед Рождеством",
	5: "Дайте мне Бензопилу",
}

func GetTrack(album int, track int, rep *database.PostgresRepository) database.Audio {
	audio, err := rep.GetAudioByAlbumIDAndTrackNumber(album, track)
	if err != nil {
		log.Printf("failed to get audio by album ID %d and track number %d: %v", album, track, err)
		return database.Audio{
			ID: 0,
		}
	}
	return audio
}
