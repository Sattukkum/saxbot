package environment

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type MainEnvironment struct {
	Token        string
	RedisHost    string
	RedisPort    string
	RedisDB      int
	AllowedChats []int64
	KatyaID      int64
	Admins       []int64
	QuizChatID   int64
}

type DataEnvironment struct {
	YandexLink  string
	YoutubeLink string
	VkLink      string
	DonateLink  string
	BoostLink   string
}

func GetMainEnvironment() MainEnvironment {

	allowedChats := getAllowedChats()
	admins := GetAdmins()
	redisDB := getRedisDB()
	katyaID := getKatyaID()
	quizChatID := getQuizChatID()

	return MainEnvironment{
		Token:        os.Getenv("BOT_TOKEN"),
		RedisHost:    os.Getenv("REDIS_HOST"),
		RedisPort:    os.Getenv("REDIS_PORT"),
		RedisDB:      redisDB,
		AllowedChats: allowedChats,
		KatyaID:      katyaID,
		Admins:       admins,
		QuizChatID:   quizChatID,
	}
}

func GetDataEnvironment() DataEnvironment {
	return DataEnvironment{
		YandexLink:  os.Getenv("YANDEX_LINK"),
		YoutubeLink: os.Getenv("YOUTUBE_LINK"),
		VkLink:      os.Getenv("VK_LINK"),
		DonateLink:  os.Getenv("DONATE_LINK"),
		BoostLink:   os.Getenv("BOOST_LINK"),
	}
}

func getAllowedChats() []int64 {
	allowedChats := os.Getenv("ALLOWED_CHATS")
	allowedChatsSlice := strings.Split(allowedChats, ",")
	var allowedChatsInts []int64
	for i, s := range allowedChatsSlice {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		chatID, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Printf("Ошибка парсинга чата #%d '%s': %v", i+1, s, err)
			continue
		}
		allowedChatsInts = append(allowedChatsInts, chatID)
	}
	return allowedChatsInts
}

func GetAdmins() []int64 {
	admins := os.Getenv("ADMINS")
	if admins == "" {
		log.Printf("ADMINS environment variable is empty")
		return []int64{}
	}
	adminInts := make([]int, 0)
	for s := range strings.SplitSeq(admins, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if adminInt, err := strconv.Atoi(s); err == nil {
			adminInts = append(adminInts, adminInt)
		}
	}
	adminInts64 := make([]int64, 0)
	for _, adminInt := range adminInts {
		adminInts64 = append(adminInts64, int64(adminInt))
	}
	return adminInts64
}

func getRedisDB() int {
	redisDB := os.Getenv("REDIS_DB")
	if redisDB == "" {
		log.Printf("REDIS_DB environment variable is empty")
		return 0
	}
	redisDBInt, err := strconv.Atoi(redisDB)
	if err != nil {
		log.Printf("Failed to parse REDIS_DB environment variable: %v", err)
		return 0
	}
	return redisDBInt
}

func getKatyaID() int64 {
	katyaID := os.Getenv("KATYA_ID")
	if katyaID == "" {
		log.Printf("KATYA_ID environment variable is empty")
		return 0
	}
	katyaIDInt, _ := strconv.ParseInt(strings.TrimSpace(katyaID), 10, 64)
	return katyaIDInt
}

func getQuizChatID() int64 {
	quizChatID := os.Getenv("TARGET_CHAT")
	if quizChatID == "" {
		log.Printf("TARGET_CHAT environment variable is empty")
		return int64(-1001673563051)
	}
	quizChatIDInt, _ := strconv.ParseInt(strings.TrimSpace(quizChatID), 10, 64)
	return quizChatIDInt
}
