package services

type BookingService struct {
	br BookingRepository
}

func NewBookingService(br BookingRepository) *BookingService {
	return &BookingService{br: br}
}

func (bs *BookingService) Register(email string) error {
	return bs.br.Inset(email)
}
