package environment

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type MainEnvironment struct {
	Token           string
	RedisHost       string
	RedisPort       int
	RedisDB         int
	AllowedChats    []int64
	Admins          []int64
	AdminsUsernames []string
	QuizChatID      int64
}

type PostgreSQLEnvironment struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
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
	redisPort := getRedisPort()
	quizChatID := getQuizChatID()
	adminUsernames := getAdminsUsernames()

	return MainEnvironment{
		Token:           os.Getenv("BOT_TOKEN"),
		RedisHost:       os.Getenv("REDIS_HOST"),
		RedisPort:       redisPort,
		RedisDB:         redisDB,
		AllowedChats:    allowedChats,
		Admins:          admins,
		AdminsUsernames: adminUsernames,
		QuizChatID:      quizChatID,
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

func GetPostgreSQLEnvironment() PostgreSQLEnvironment {
	portStr := os.Getenv("POSTGRES_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		port = 5432
	}

	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}

	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "saxbot"
	}

	database := os.Getenv("POSTGRES_DB")
	if database == "" {
		database = "saxbot"
	}

	sslmode := os.Getenv("POSTGRES_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		panic("POSTGRES_PASSWORD environment variable is empty")
	}

	return PostgreSQLEnvironment{
		Host:     host,
		Port:     port,
		User:     user,
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Database: database,
		SSLMode:  sslmode,
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

func getAdminsUsernames() []string {
	adminsUsernames := os.Getenv("ADMINS_USERNAMES")
	if adminsUsernames == "" {
		log.Println("ADMINS_USERNAMES environment variable is empty")
		return []string{}
	}
	adminsUsernamesSlice := strings.Split(adminsUsernames, ",")
	return adminsUsernamesSlice
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

func getRedisPort() int {
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		log.Printf("REDIS_PORT environment variable is empty")
		return 0
	}
	redisPortInt, err := strconv.Atoi(redisPort)
	if err != nil {
		log.Printf("Failed to parse REDIS_PORT environment variable: %v", err)
		return 0
	}
	return redisPortInt
}

func getQuizChatID() int64 {
	quizChatID := os.Getenv("TARGET_CHAT")
	if quizChatID == "" {
		log.Printf("TARGET_CHAT environment variable is empty")
		return 0
	}
	quizChatIDInt, _ := strconv.ParseInt(strings.TrimSpace(quizChatID), 10, 64)
	return quizChatIDInt
}
