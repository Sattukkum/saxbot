package database

import "saxbot/domain"

func userFromPostgresToDomain(up *UserPostgres) domain.User {
	return domain.User{
		UserID:    up.UserID,
		FirstName: up.FirstName,
		Username:  up.Username,
		IsAdmin:   up.IsAdmin,
		Warns:     up.Warns,
		Status:    up.Status,
		IsWinner:  up.IsWinner,
		AdminPref: up.AdminPref,
	}
}

func userFromDomainToPostgres(u *domain.User) UserPostgres {
	return UserPostgres{
		UserID:    u.UserID,
		FirstName: u.FirstName,
		Username:  u.Username,
		IsAdmin:   u.IsAdmin,
		Warns:     u.Warns,
		Status:    u.Status,
		IsWinner:  u.IsWinner,
		AdminPref: u.AdminPref,
	}
}

func quizFromPostgresToDomain(qp *QuizPostgres) domain.Quiz {
	return domain.Quiz{
		Date:     qp.Date,
		Song:     qp.SongName,
		Quote:    qp.Quote,
		QuizTime: qp.QuizTime,
		IsActive: qp.IsActive,
	}
}

func quizFromDomainToPostgres(q *domain.Quiz) QuizPostgres {
	return QuizPostgres{
		Date:     q.Date,
		SongName: q.Song,
		Quote:    q.Quote,
		QuizTime: q.QuizTime,
		IsActive: q.IsActive,
	}
}
