package services

import models "github.com/SokolovAS/bookingprocessor/internal/Models"

// UserRepository defines the persistence operations required by the service.
type UserRepository interface {
	Create(user models.User) (int, error)
	GetAll() ([]models.User, error)
}
