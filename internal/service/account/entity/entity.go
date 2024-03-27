package entity

import (
	"database/sql"
	"time"
)

type AccountKind uint8

const (
	AK_EMAIL_GEN = AccountKind(1) + iota
	AK_PRIVATE_OWN
)

type Account struct {
	WalletAddress string       `gorm:"primary_key;size:64" json:"walletAddress"`
	Kind          AccountKind  `gorm:"not null" json:"kind"`
	Email         *string      `gorm:"default:null;size:50" json:"email"`
	PublicKey     string       `gorm:"not null;size:64" json:""`
	Seed          *string      `gorm:"default:null" json:"-"`
	CreatedAt     time.Time    `gorm:"comment:create time" json:"createdAt"`
	UpdatedAt     sql.NullTime `gorm:"comment:update time" json:"updatedAt"`
}
