package models

import "time"

// Follow 关注关系表
type Follow struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FollowerID  uint      `gorm:"not null;uniqueIndex:uk_follow;index:idx_follower" json:"follower_id"`
	FollowedID  uint      `gorm:"not null;uniqueIndex:uk_follow;index:idx_followed" json:"followed_id"`
	CreatedAt   time.Time `json:"created_at"`

	Follower *User `gorm:"foreignKey:FollowerID" json:"follower,omitempty"`
	Followed *User `gorm:"foreignKey:FollowedID" json:"followed,omitempty"`
}

func (Follow) TableName() string { return "follows" }

func CreateFollow(followerID, followedID uint) error {
	return DB.Create(&Follow{FollowerID: followerID, FollowedID: followedID}).Error
}

func DeleteFollow(followerID, followedID uint) error {
	return DB.Where("follower_id = ? AND followed_id = ?", followerID, followedID).Delete(&Follow{}).Error
}

func IsFollowing(followerID, followedID uint) bool {
	var count int64
	DB.Model(&Follow{}).Where("follower_id = ? AND followed_id = ?", followerID, followedID).Count(&count)
	return count > 0
}

func GetFollowingCount(userID uint) int64 {
	var count int64
	DB.Model(&Follow{}).Where("follower_id = ?", userID).Count(&count)
	return count
}

func GetFollowersCount(userID uint) int64 {
	var count int64
	DB.Model(&Follow{}).Where("followed_id = ?", userID).Count(&count)
	return count
}

// GetFollowingList 我关注的人
func GetFollowingList(userID uint, page, pageSize int) ([]User, int64, error) {
	var total int64
	DB.Model(&Follow{}).Where("follower_id = ?", userID).Count(&total)

	var users []User
	offset := (page - 1) * pageSize
	DB.Table("users").
		Joins("INNER JOIN follows ON follows.followed_id = users.id").
		Where("follows.follower_id = ?", userID).
		Order("follows.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&users)
	return users, total, nil
}

// GetFollowerList 关注我的人
func GetFollowerList(userID uint, page, pageSize int) ([]User, int64, error) {
	var total int64
	DB.Model(&Follow{}).Where("followed_id = ?", userID).Count(&total)

	var users []User
	offset := (page - 1) * pageSize
	DB.Table("users").
		Joins("INNER JOIN follows ON follows.follower_id = users.id").
		Where("follows.followed_id = ?", userID).
		Order("follows.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&users)
	return users, total, nil
}
