package collab

// ==================== 主笔转让 ====================
// 语义：作品主笔（Novel.AuthorID）换人。这是**不可逆的高风险操作**，所以：
//   1. 只有现任主笔本人或平台可发起；
//   2. 受让人必须是已接受的协作者（陌生人必须先进协作，避免转错人）；
//   3. 事务内完成「换主笔 + 原主笔降为编辑」两步，避免中间态出现「无主笔」；
//   4. 审计记录 before/after，可追溯。
//
// 不自动转让收益/结算数据：收益按历史流水归属，改主笔不追溯已产生的分成（见 docs）。

import (
	"errors"
	"strconv"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// ErrTransferSelf 不能转给自己
var ErrTransferSelf = errors.New("不能把主笔转让给自己")

// Transfer 转让主笔（actorID 必须是现任主笔或平台）
func Transfer(novelID, actorID, toUserID uint, isPlatform bool) error {
	if novelID == 0 || toUserID == 0 {
		return errors.New("参数错误")
	}
	novel, err := models.GetNovelByID(novelID)
	if err != nil || novel == nil {
		return errors.New("作品不存在")
	}
	if actorID != novel.AuthorID && !isPlatform {
		return errors.New("只有现任主笔或平台管理员可以转让主笔")
	}
	if toUserID == novel.AuthorID {
		return ErrTransferSelf
	}
	// 受让人必须是已接受的协作者
	member, merr := Get(novelID, toUserID)
	if merr != nil || member == nil || member.Status != StatusActive {
		return errors.New("受让人必须是该作品已接受的协作者（先邀请并接受再转让）")
	}
	target, terr := models.GetUserByID(toUserID)
	if terr != nil || target == nil {
		return errors.New("受让人不存在")
	}

	oldOwnerID := novel.AuthorID
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		// 1) 换主笔
		if err := tx.Model(&models.Novel{}).Where("id = ?", novelID).
			Updates(map[string]interface{}{"author_id": toUserID, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		// 2) 新主笔的协作者记录移除（主笔身份由 Novel.AuthorID 体现，避免双重身份）
		if err := tx.Where("novel_id = ? AND user_id = ?", novelID, toUserID).
			Delete(&Collaborator{}).Error; err != nil {
			return err
		}
		// 3) 原主笔降为编辑（保留改他人章节的能力，避免接手即失去权限）
		oldRec, gerr := getRaw(tx, novelID, oldOwnerID)
		if gerr == nil && oldRec != nil {
			if err := tx.Model(&Collaborator{}).Where("id = ?", oldRec.ID).Updates(map[string]interface{}{
				"role": RoleEditor, "status": StatusActive,
				"can_edit_others": true, "updated_at": time.Now(),
			}).Error; err != nil {
				return err
			}
		} else {
			rec := &Collaborator{
				NovelID: novelID, UserID: oldOwnerID, Role: RoleEditor, Status: StatusActive,
				CanEditOthers: true, BylineOrder: 100,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if err := tx.Create(rec).Error; err != nil {
				return err
			}
		}
		// 4) 审计
		return tx.Create(&Audit{
			NovelID: novelID, ActorID: actorID, Action: "transfer_owner",
			TargetUserID: toUserID,
			Before:       "owner=" + uitoa(oldOwnerID),
			After:        "owner=" + uitoa(toUserID) + ";old=editor",
			CreatedAt:    time.Now(),
		}).Error
	})
	return err
}

// getRaw 取原始协作记录（事务内用）
func getRaw(tx *gorm.DB, novelID, userID uint) (*Collaborator, error) {
	var m Collaborator
	if err := tx.Where("novel_id = ? AND user_id = ?", novelID, userID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func uitoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }

// CanTransfer 是否可发起转让（供前端显隐按钮）
func CanTransfer(novel *models.Novel, userID uint, isPlatform bool) bool {
	if novel == nil || userID == 0 {
		return false
	}
	if isPlatform {
		return true
	}
	return novel.AuthorID == userID
}
