package repository

import (
	"database/sql"
)

type UserRepo struct {
	db *sql.DB
}

func NewRepo() services.UserReposo
