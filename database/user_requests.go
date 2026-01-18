package database

import (
	"fmt"
	"log"
	"time"

	"saxbot/environment"
	"slices"

	"gorm.io/gorm"
)

// Получить пользователя по Telegram UserID, создает нового если не найден
func (p *PostgresRepository) GetUser(userID int64) (User, error) {
	var user User
	err := p.db.Where("user_id = ?", userID).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("Creating new user %d (not found in database)", userID)

			newUser := User{
				UserID:    userID,
				FirstName: "",
				Username:  "",
				Warns:     0,
				Status:    "active",
			}

			err = p.db.Create(&newUser).Error
			if err != nil {
				return User{}, fmt.Errorf("failed to create user: %w", err)
			}

			log.Printf("Created new user %d", userID)
			return newUser, nil
		}
		log.Printf("Failed to get user %d, %v", userID, err)
		return User{}, fmt.Errorf("failed to get user: %w", err)
	}
	log.Printf("Got user %d from Postgres\nParams:\nUsername:%s\nWarns:%d", user.UserID, user.Username, user.Warns)
	return user, nil
}

// Получить пользователя по username
func (p *PostgresRepository) GetUserByUsername(username string) (*User, error) {
	var user User
	err := p.db.Where("username = ?", username).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return &user, nil
}

// Получить всех пользователей из базы данных
func (p *PostgresRepository) GetAllUsers() ([]User, error) {
	var users []User
	err := p.db.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	return users, nil
}

// Сохранить пользователя в базе данных
func (p *PostgresRepository) SaveUser(user *User) error {
	return p.db.Save(user).Error
}

// Обновить количество предупреждений пользователя
func (p *PostgresRepository) UpdateUserWarns(userID int64, delta int) error {
	user, err := p.GetUser(userID)
	if err != nil {
		return err
	}

	user.Warns += delta
	if user.Warns < 0 {
		user.Warns = 0
	}

	return p.SaveUser(&user)
}

// Обновить username пользователя
func (p *PostgresRepository) UpdateUsername(userID int64, username string) error {
	result := p.db.Model(&User{}).Where("user_id = ?", userID).Update("username", username)
	if result.Error != nil {
		return fmt.Errorf("failed to update username: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", userID)
	}
	return nil
}

// Обновить админский статус пользователя на основе переменной окружения ADMINS
func (p *PostgresRepository) IsUserAdmin(user *User) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, user.UserID)
	return newAdminStatus
}

// Обновить админский статус для всех пользователей
func (p *PostgresRepository) RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for all users...")

	users, err := p.GetAllUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	updatedCount := 0
	errCount := 0
	for _, user := range users {
		err = nil
		isAdmin := p.IsUserAdmin(&user)
		if isAdmin && !p.IsAdmin(user.UserID) {
			err = p.SaveAdmin(user, "junior")
			updatedCount++
		} else if !isAdmin && p.IsAdmin(user.UserID) {
			err = p.RemoveAdmin(user.UserID)
			updatedCount++
		}
		if err != nil {
			log.Printf("failed to update admin status for user %d: %v", user.UserID, err)
			errCount++
		}
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(users))
	if errCount != 0 {
		return fmt.Errorf("got %d errors during update", errCount)
	}
	return nil
}

// Получить всех админов
func (p *PostgresRepository) GetAdmins() ([]Admin, error) {
	var admins []Admin
	err := p.db.Find(&admins).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all admins: %w", err)
	}
	return admins, nil
}

// Сохранить админа
func (p *PostgresRepository) SaveAdmin(user User, adminRole string) error {
	admin := Admin{
		ID:        user.UserID,
		User:      user,
		AdminRole: adminRole,
	}
	err := p.db.Save(&admin).Error
	if err != nil {
		return fmt.Errorf("failed to set user as admin: %w", err)
	}
	log.Printf("User %d successfully set as admin", user.UserID)
	return nil
}

// Проверить, является ли пользователь админом
func (p *PostgresRepository) IsAdmin(userID int64) bool {
	var admin Admin
	err := p.db.Where("id = ?", userID).First(&admin).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Printf("failed to figure out if user %d is admin: %v", userID, err)
	}
	return err == nil
}

// Получить роль админа
func (p *PostgresRepository) GetAdminRole(userID int64) (string, error) {
	var admin Admin
	err := p.db.Where("id = ?", userID).First(&admin).Error
	if err != nil {
		return "", fmt.Errorf("failed to get %d admin role: %w", userID, err)
	}
	return admin.AdminRole, nil
}

// Удалить админа
func (p *PostgresRepository) RemoveAdmin(userID int64) error {
	var admin Admin
	err := p.db.Where("id = ?", userID).Delete(&admin).Error
	if err != nil {
		return fmt.Errorf("failed to remove user %d from admins: %w", userID, err)
	}
	log.Printf("User %d successfully removed from admins", userID)
	return nil
}

// Продвинуть админа
func (p *PostgresRepository) PromoteAdmin(userID int64, delta string) error {
	var admin Admin
	err := p.db.Where("id = ?", userID).First(&admin).Error
	if err != nil {
		return fmt.Errorf("failed to get %d from admins: %w", userID, err)
	}
	if delta == "+" && admin.AdminRole == "junior" {
		admin.AdminRole = "senior"
	} else if delta == "-" && admin.AdminRole == "senior" {
		admin.AdminRole = "junior"
	} else {
		return fmt.Errorf("delta %s can't be used to change role %s", delta, admin.AdminRole)
	}
	err = p.SaveAdmin(admin.User, admin.AdminRole)
	if err != nil {
		return err
	}
	return nil
}

// Обновить количество сообщений пользователя
func (p *PostgresRepository) UpdateUserMessageCount(userID int64, delta int) error {
	user, err := p.GetUser(userID)
	if err != nil {
		return err
	}
	user.MessageCount += delta
	return p.SaveUser(&user)
}

// Получить количество сообщений пользователя
func (p *PostgresRepository) GetUserMessageCount(userID int64) (int, error) {
	user, err := p.GetUser(userID)
	if err != nil {
		return 0, err
	}
	return user.MessageCount, nil
}

// Очистить количество сообщений у всех пользователей
func (p *PostgresRepository) ClearAllUsersMessageCount() error {
	err := p.db.Model(&User{}).Update("message_count", 0).Error
	if err != nil {
		return fmt.Errorf("failed to clear all users message count: %w", err)
	}
	return nil
}

// Получить топ 5 пользователей с наибольшим количеством сообщений
func (p *PostgresRepository) GetUsersWithTopActivity() ([]User, error) {
	var users []User
	err := p.db.Order("message_count DESC").Limit(5).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with top activity: %w", err)
	}
	return users, nil
}

func (p *PostgresRepository) GetUserBirthday(userID int64) (time.Time, error) {
	var user User
	err := p.db.Where("user_id = ?", userID).First(&user).Error
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get user birthday: %w", err)
	}
	return user.Birthday, nil
}

func (p *PostgresRepository) UpdateUserBirthday(userID int64, birthday time.Time) error {
	user, err := p.GetUser(userID)
	if err != nil {
		return err
	}
	user.Birthday = birthday
	return p.SaveUser(&user)
}

func (p *PostgresRepository) GetUsersWithBirthdayToday() ([]User, error) {
	var users []User

	now := time.Now().In(MoscowTZ)
	month := int(now.Month())
	day := now.Day()

	isLeapYear := func(y int) bool {
		if y%400 == 0 {
			return true
		}
		if y%100 == 0 {
			return false
		}
		return y%4 == 0
	}

	query := p.db.
		Where(
			`
			birthday IS NOT NULL
			AND EXTRACT(YEAR FROM birthday) > 1900
			AND status != 'banned'
			AND (
				(EXTRACT(MONTH FROM birthday) = ? AND EXTRACT(DAY FROM birthday) = ?)
			)
			`,
			month, day,
		)

	// 29 февраля → поздравляем 28 февраля в невисокосный год
	if month == 2 && day == 28 && !isLeapYear(now.Year()) {
		query = query.Or(
			`
			birthday IS NOT NULL
			AND EXTRACT(YEAR FROM birthday) > 1900
			AND status != 'banned'
			AND EXTRACT(MONTH FROM birthday) = 2
			AND EXTRACT(DAY FROM birthday) = 29
			`,
		)
	}

	err := query.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with birthday today: %w", err)
	}

	return users, nil
}

// Получить время размута пользователя
func (p *PostgresRepository) GetUserMutedUntil(userID int64) (time.Time, error) {
	user, err := p.GetUser(userID)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to ger user %d: %w", userID, err)
	}
	return user.MutedUntil, nil
}

// Сохранить время размута пользователя
func (p *PostgresRepository) SaveUserMutedUntil(userID int64, minutes uint) error {
	if minutes == 0 {
		return fmt.Errorf("minutes must be positive, got %d", minutes)
	}
	now := time.Now().In(MoscowTZ)
	mutedUntil := now.Add(time.Duration(minutes) * time.Minute)

	user, err := p.GetUser(userID)
	if err != nil {
		return fmt.Errorf("failed to get user %d: %w", userID, err)
	}

	user.MutedUntil = mutedUntil
	return p.SaveUser(&user)
}

// Получить всех пользователей, которых пора размутить
func (p *PostgresRepository) GetAllMutedToUnmute() ([]User, error) {
	var users []User
	now := time.Now().In(MoscowTZ)
	query := p.db.Where(
		`status = 'muted'
		AND muted_until IS NOT NULL
		AND muted_until < ?`,
		now,
	)

	err := query.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users to unmute: %w", err)
	}

	return users, nil
}

func (p *PostgresRepository) GetAllMutedUsers() ([]User, error) {
	var users []User
	err := p.db.Where(`status = ?`, "muted").Find(&users).Error
	if err != nil {
		return []User{}, fmt.Errorf("failed to get all muted users: %w", err)
	}
	return users, nil
}

func (p *PostgresRepository) GetAllRestrictedUsers() ([]User, error) {
	var users []User
	err := p.db.Where(`status = ?`, "restricted").Find(&users).Error
	if err != nil {
		return []User{}, fmt.Errorf("failed to get all restricted users: %w", err)
	}
	return users, nil
}
