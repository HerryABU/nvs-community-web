package models

import (
	"time"

	"gorm.io/gorm"
)

// DB 全局数据库实例
var DB *gorm.DB

type User struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Username        string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Email           string     `gorm:"size:128;uniqueIndex;not null" json:"email"`
	PasswordHash    string     `gorm:"size:256;not null" json:"-"`
	Nickname        string     `gorm:"size:64;default:''" json:"nickname"`
	AvatarURL       string     `gorm:"size:512;default:''" json:"avatar_url"`
	Bio             string     `gorm:"type:text" json:"bio"`
	Role            string     `gorm:"size:32;default:reader" json:"role"`
	SigningKey      string     `gorm:"size:64;default:''" json:"-"` // base64(AES-256 key), only set for authors
	RealNameVerified bool      `gorm:"default:false" json:"real_name_verified"`
	LoginFailCount  int        `gorm:"default:0" json:"-"`
	LockedUntil     *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

func CreateUser(user *User) error {
	return DB.Create(user).Error
}

func GetUserByEmail(email string) (*User, error) {
	var user User
	err := DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByID(id uint) (*User, error) {
	var user User
	err := DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func UpdateUser(user *User) error {
	return DB.Save(user).Error
}
