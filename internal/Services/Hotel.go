package services

import (
	models "github.com/SokolovAS/bookingprocessor/internal/Models"
	"log"
)

type HotelService struct {
	HotelRepo HotelRepository
}

func NewHotelService(hotelRepo HotelRepository) *HotelService {
	return &HotelService{HotelRepo: hotelRepo}
}

func (r HotelService) Create(userID int) error {
	hotel := models.Hotel{
		UserID: userID,
		Data:   "Simple Hotel Data",
	}
	err := r.HotelRepo.Create(hotel)
	if err != nil {
		log.Printf("Insert into hotels failed: %v", err)
		return err
	}
	return nil
}
