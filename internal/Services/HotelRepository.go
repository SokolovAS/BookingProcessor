package services

import (
	"database/sql"
)

type HotelRepository interface {
	CreateTx(tx *sql.Tx, userid int) error
}
