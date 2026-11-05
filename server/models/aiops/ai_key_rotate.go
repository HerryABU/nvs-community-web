package aiops

import (
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"nvs-server/utils/secure"
)

// ReencryptAiModelKeys 把全部上游 API Key 从旧密钥迁移到新密钥（换令牌加密密码时调用）。
//
// 语义保证：**全有或全无** —— 先全部解密成功再统一写回；
// 任何一条解不开就整体放弃并报错，绝不出现「一半新钥匙一半旧钥匙」导致模型全挂。
func ReencryptAiModelKeys(oldKey, newKey []byte) (int, error) {
	if len(oldKey) != 32 || len(newKey) != 32 {
		return 0, secure.ErrFieldCryptoNoKey
	}
	var list []ai.AiModel
	if err := models.DB.Where("api_key_enc != ''").Find(&list).Error; err != nil {
		return 0, err
	}
	// 第一阶段：全部解密 + 重新加密（只写内存）
	type pending struct {
		id  uint
		enc string
	}
	todo := make([]pending, 0, len(list))
	for i := range list {
		plain, err := secure.DecryptFieldWithKey(oldKey, list[i].ApiKeyEnc)
		if err != nil {
			return 0, err // 解不开就中止，不做任何写入
		}
		enc, err := secure.EncryptFieldWithKey(newKey, plain)
		if err != nil {
			return 0, err
		}
		todo = append(todo, pending{id: list[i].ID, enc: enc})
	}
	// 第二阶段：统一落库
	n := 0
	for _, t := range todo {
		if err := models.DB.Model(&ai.AiModel{}).Where("id = ?", t.id).
			Update("api_key_enc", t.enc).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
