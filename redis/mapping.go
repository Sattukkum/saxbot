package redis

import "saxbot/domain"

func userFromRedisToDomain(userID int64, ur UserRedis) domain.User {
	return domain.User{
		UserID:    userID,
		FirstName: ur.FirstName,
		Username:  ur.Username,
		IsAdmin:   ur.IsAdmin,
		Warns:     ur.Warns,
		Status:    ur.Status,
		IsWinner:  ur.IsWinner,
		AdminPref: ur.AdminPref,
	}
}

func userFromDomainToRedis(u domain.User) UserRedis {
	return UserRedis{
		FirstName: u.FirstName,
		Username:  u.Username,
		IsAdmin:   u.IsAdmin,
		Warns:     u.Warns,
		Status:    u.Status,
		IsWinner:  u.IsWinner,
		AdminPref: u.AdminPref,
	}
}
