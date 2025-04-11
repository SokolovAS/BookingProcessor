package Handlers

import (
	"fmt"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
	"github.com/google/uuid"
	"log"
	"net/http"
)

type BookingHandler struct {
	bs *services.BookingService
}

func NewBookingHandler(bs *services.BookingService) *BookingHandler {
	return &BookingHandler{bs: bs}
}

func (bh *BookingHandler) Inset(response http.ResponseWriter, request *http.Request) {
	email := fmt.Sprintf("john+%s@example.com", uuid.New().String())
	err := bh.bs.Register(email)
	if err != nil {
		log.Fatalf("failed to register user: %v", err)
	}
	response.Write([]byte("Data inserted successfully"))
}
