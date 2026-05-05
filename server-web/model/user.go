package model

import "time"

type User struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"column:password;type:varchar(255);not null" json:"-"`
	Role      string    `gorm:"type:varchar(32);not null;default:viewer" json:"role"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
