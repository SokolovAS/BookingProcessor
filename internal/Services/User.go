package services

import (
	models "github.com/SokolovAS/bookingprocessor/internal/Models"
)

type UserService struct {
	UserRepo UserRepository
}

func NewUserService(ur UserRepository) *UserService {
	return &UserService{ur}
}

func (us *UserService) Register(name, email string) (models.User, error) {
	panic("implement me")
}

func (us *UserService) List() ([]models.User, error) {
	return us.UserRepo.GetAll()
}
