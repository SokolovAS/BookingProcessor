package services

type HotelService struct {
	HotelRepo HotelRepository
}

func NewHotelService(hotelRepo HotelRepository) *HotelService {
	return &HotelService{HotelRepo: hotelRepo}
}

func (r HotelService) Create(userID int) error {
	panic("implement me")
}
