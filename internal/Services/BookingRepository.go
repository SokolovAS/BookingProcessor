package services

type BookingRepository interface {
	Inset(email string) error
}
