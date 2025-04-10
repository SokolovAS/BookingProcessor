package services

import models "github.com/SokolovAS/bookingprocessor/internal/Models"

type HotelRepository interface {
	Create(hotel models.Hotel) error
}
