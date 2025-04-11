package services

import (
	"database/sql"
	models "github.com/SokolovAS/bookingprocessor/internal/Models"
)

// UserRepository defines the persistence operations required by the service.
type UserRepository interface {
	CreateTX(tx *sql.Tx, email string) (int, error)
	GetAll() ([]models.User, error)
}
