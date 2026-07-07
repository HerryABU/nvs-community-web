package models

import "time"

type Rating struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	UserID              uint      `gorm:"not null;uniqueIndex:uk_user_novel_rating" json:"user_id"`
	NovelID             uint      `gorm:"not null;uniqueIndex:uk_user_novel_rating;index" json:"novel_id"`
	TypeCompletion      int       `gorm:"default:3" json:"type_completion"`
	NarrativeQuality    int       `gorm:"default:3" json:"narrative_quality"`
	ThoughtDepth        int       `gorm:"default:3" json:"thought_depth"`
	CommunityReputation int       `gorm:"default:3" json:"community_reputation"`
	UpdateStability     int       `gorm:"default:3" json:"update_stability"`
	CreatedAt           time.Time `json:"created_at"`
	User                *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Rating) TableName() string { return "ratings" }

func UpsertRating(r *Rating) error {
	var existing Rating
	err := DB.Where("user_id = ? AND novel_id = ?", r.UserID, r.NovelID).First(&existing).Error
	if err != nil {
		return DB.Create(r).Error
	}
	DB.Model(&existing).Updates(map[string]interface{}{
		"type_completion":      r.TypeCompletion,
		"narrative_quality":    r.NarrativeQuality,
		"thought_depth":        r.ThoughtDepth,
		"community_reputation": r.CommunityReputation,
		"update_stability":     r.UpdateStability,
	})
	// 重新查询返回完整数据
	return DB.First(r, existing.ID).Error
}

func GetRatingByUserNovel(userID, novelID uint) (*Rating, error) {
	var r Rating
	err := DB.Where("user_id = ? AND novel_id = ?", userID, novelID).First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func GetRatingStats(novelID uint) (avg map[string]float64, count int64, err error) {
	var result struct {
		AvgTypeCompletion      float64
		AvgNarrativeQuality    float64
		AvgThoughtDepth        float64
		AvgCommunityReputation float64
		AvgUpdateStability     float64
		Count                  int64
	}
	err = DB.Model(&Rating{}).
		Select("COALESCE(AVG(type_completion),0) as avg_type_completion, "+
			"COALESCE(AVG(narrative_quality),0) as avg_narrative_quality, "+
			"COALESCE(AVG(thought_depth),0) as avg_thought_depth, "+
			"COALESCE(AVG(community_reputation),0) as avg_community_reputation, "+
			"COALESCE(AVG(update_stability),0) as avg_update_stability, "+
			"COUNT(*) as count").
		Where("novel_id = ?", novelID).
		Scan(&result).Error
	if err != nil {
		return nil, 0, err
	}
	return map[string]float64{
		"type_completion":      result.AvgTypeCompletion,
		"narrative_quality":    result.AvgNarrativeQuality,
		"thought_depth":        result.AvgThoughtDepth,
		"community_reputation": result.AvgCommunityReputation,
		"update_stability":     result.AvgUpdateStability,
	}, result.Count, nil
}

// IsAuthorPeer 检查评分者是否是作者
func IsAuthorPeer(userID uint) bool {
	var count int64
	DB.Model(&Novel{}).Where("author_id = ? AND status = ?", userID, "published").Count(&count)
	return count > 0
}
