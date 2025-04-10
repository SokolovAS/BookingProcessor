package services

import (
	models "github.com/SokolovAS/bookingprocessor/internal/Models"
	"time"
)

type UserService struct {
	UserRepo UserRepository
}

func NewUserService(ur UserRepository) *UserService {
	return &UserService{ur}
}

func (us *UserService) Register(name, email string) (models.User, error) {
	newUser := models.User{
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}
	id, err := us.UserRepo.Create(newUser)
	if err != nil {
		return models.User{}, err
	}
	newUser.ID = id
	return newUser, nil
}

func (us *UserService) List() ([]models.User, error) {
	return us.UserRepo.GetAll()
}
