package repository

import (
	"database/sql"

	models "github.com/SokolovAS/bookingprocessor/internal/Models"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) services.UserRepository {
	return &UserRepo{
		db: db,
	}
}

func (r *UserRepo) CreateTX(tx *sql.Tx, email string) (int, error) {
	var id int
	err := tx.QueryRow(`
		INSERT INTO users (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, email, "John Dou").Scan(&id)
	return id, err
}

func (r *UserRepo) GetAll() ([]models.User, error) {

	rows, err := r.db.Query("SELECT id, name, email, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
