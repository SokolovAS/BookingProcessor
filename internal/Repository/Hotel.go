package repository

import (
	"database/sql"
	"fmt"
	"log"

	services "github.com/SokolovAS/bookingprocessor/internal/Services"
)

type HotelRepository struct {
	db *sql.DB
}

func NewHotelRepository(db *sql.DB) services.HotelRepository {
	return &HotelRepository{db: db}
}

func (r *HotelRepository) CreateTx(tx *sql.Tx, userid int) error {
	_, err := tx.Exec("INSERT INTO hotels (user_id, data) VALUES ($1, $2);", userid, "Some data")
	if err != nil {
		log.Printf("Insert into hotels failed: %v", err)
		return fmt.Errorf("error %v", err)
	}
	return nil
}
