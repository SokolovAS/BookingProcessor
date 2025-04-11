package repository

import (
	"database/sql"
	"fmt"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
)

type BookingRepo struct {
	db        *sql.DB
	UserRepo  services.UserRepository
	HotelRepo services.HotelRepository
}

func NewBookingRepo(db *sql.DB, ur services.UserRepository, hr services.HotelRepository) *BookingRepo {
	return &BookingRepo{
		db:        db,
		UserRepo:  ur,
		HotelRepo: hr,
	}
}

func (br *BookingRepo) Inset(email string) error {
	tx, err := br.db.Begin()
	if err != nil {
		panic(err.Error())
		return err
	}
	userId, err := br.UserRepo.CreateTX(tx, email)
	if err != nil {
		panic(err.Error())
		return err
	}
	err = br.HotelRepo.CreateTx(tx, userId)
	if err != nil {
		mess, _ := fmt.Printf("error %v", err.Error())
		panic(mess)
		return err
	}
	if err = tx.Commit(); err != nil {
		panic(err.Error())
		return err
	}
	return nil
}
