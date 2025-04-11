package repository

import (
	"database/sql"
	"fmt"
	"log"

	models "github.com/SokolovAS/bookingprocessor/internal/Models"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
)

type HotelRepository struct {
	db *sql.DB
}

func NewHotelRepository(db *sql.DB) services.HotelRepository {
	return &HotelRepository{db: db}
}

func (r *HotelRepository) Create(hotel models.Hotel) error {
	err := r.db.QueryRow("INSERT INTO hotels (user_id, data) VALUES ($1, $2);", hotel.UserID, hotel.Data)
	if err != nil {
		log.Printf("Insert into hotels failed: %v", err)
		return fmt.Errorf("error %v", err)
	}
	return nil
}
