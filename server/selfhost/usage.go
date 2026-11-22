package selfhost

// 按项目按天统计代理流量与请求数；存储体积按落盘文件实时计算。

import (
	"os"
	"sort"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

type UsageDaily struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;uniqueIndex:idx_selfhost_usage_day" json:"project_id"`
	Day       string    `gorm:"size:10;not null;uniqueIndex:idx_selfhost_usage_day" json:"day"`
	Requests  int64     `gorm:"default:0" json:"requests"`
	BytesUp   int64     `gorm:"default:0" json:"bytes_up"`
	BytesDown int64     `gorm:"default:0" json:"bytes_down"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UsageDaily) TableName() string { return "selfhost_usage_daily" }

func MigrateUsage(db *gorm.DB) error { return db.AutoMigrate(&UsageDaily{}) }

func RecordTraffic(projectID uint, up, down int64) {
	if projectID == 0 {
		return
	}
	day := time.Now().Format("2006-01-02")
	models.DB.Table("selfhost_usage_daily").
		Where("project_id = ? AND day = ?", projectID, day).
		Updates(map[string]interface{}{
			"requests":   gorm.Expr("requests + 1"),
			"bytes_up":   gorm.Expr("bytes_up + ?", up),
			"bytes_down": gorm.Expr("bytes_down + ?", down),
			"updated_at": time.Now(),
		})
	// 首次请求：上面的 Updates 影响 0 行，插入初始行。
	var n int64
	models.DB.Model(&UsageDaily{}).Where("project_id = ? AND day = ?", projectID, day).Count(&n)
	if n == 0 {
		_ = models.DB.Create(&UsageDaily{ProjectID: projectID, Day: day, Requests: 1, BytesUp: up, BytesDown: down}).Error
	}
}

// StorageBytes 计算项目源码+产物实际落盘体积（字节）。
func StorageBytes(p *Project) int64 {
	var total int64
	for _, rel := range []string{p.SourceFile, p.BinaryFile} {
		if rel == "" {
			continue
		}
		abs, err := AbsStoredPath(rel)
		if err != nil {
			continue
		}
		if st, err := os.Stat(abs); err == nil {
			total += st.Size()
		}
	}
	return total
}

type UsageRow struct {
	ProjectID    uint   `json:"project_id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	OwnerID      uint   `json:"owner_id"`
	StorageBytes int64  `json:"storage_bytes"`
	Requests     int64  `json:"requests"`
	BytesUp      int64  `json:"bytes_up"`
	BytesDown    int64  `json:"bytes_down"`
	Days         int    `json:"days"`
}

// ListUsage 全站项目用量（含未产生流量的项目，便于核对体积）。
func ListUsage() ([]UsageRow, error) {
	var projects []Project
	if err := models.DB.Select("id", "user_id", "name", "slug", "source_file", "binary_file").Find(&projects).Error; err != nil {
		return nil, err
	}
	var daily []UsageDaily
	_ = models.DB.Find(&daily).Error
	agg := map[uint]*UsageRow{}
	for _, d := range daily {
		r := agg[d.ProjectID]
		if r == nil {
			r = &UsageRow{ProjectID: d.ProjectID}
			agg[d.ProjectID] = r
		}
		r.Requests += d.Requests
		r.BytesUp += d.BytesUp
		r.BytesDown += d.BytesDown
		r.Days++
	}
	out := make([]UsageRow, 0, len(projects))
	for _, p := range projects {
		r := UsageRow{ProjectID: p.ID, Name: p.Name, Slug: p.Slug, OwnerID: p.UserID, StorageBytes: StorageBytes(&p)}
		if a := agg[p.ID]; a != nil {
			r.Requests, r.BytesUp, r.BytesDown, r.Days = a.Requests, a.BytesUp, a.BytesDown, a.Days
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BytesUp+out[i].BytesDown > out[j].BytesUp+out[j].BytesDown })
	return out, nil
}

// ProjectUsage 单项目用量（数据库聚合 + 实时体积）。
func ProjectUsage(projectID uint) UsageRow {
	row := UsageRow{ProjectID: projectID}
	if p, err := GetProjectRaw(projectID); err == nil {
		row.Name, row.Slug, row.OwnerID = p.Name, p.Slug, p.UserID
		row.StorageBytes = StorageBytes(p)
	}
	models.DB.Model(&UsageDaily{}).Where("project_id = ?", projectID).
		Select("COALESCE(SUM(requests),0)", "COALESCE(SUM(bytes_up),0)", "COALESCE(SUM(bytes_down),0)", "COUNT(*)").
		Row().Scan(&row.Requests, &row.BytesUp, &row.BytesDown, &row.Days)
	return row
}
