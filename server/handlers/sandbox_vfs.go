package handlers

import (
	"fmt"
	"sync"

	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 沙盒虚拟文件系统 (VFS) ====================
// 每个沙盒应用拥有独立的内存存储空间，通过API读写
// 隔离于真实文件系统，防止沙盒逃逸

// VFSStore 虚拟文件系统存储（按沙盒ID隔离）
type VFSStore struct {
	mu   sync.RWMutex
	data map[uint]map[string]string // htmlId → key → value
}

var GlobalVFS = &VFSStore{
	data: make(map[uint]map[string]string),
}

// getBucket 获取沙盒的存储桶
func (v *VFSStore) getBucket(htmlID uint) map[string]string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.data[htmlID] == nil {
		v.data[htmlID] = make(map[string]string)
	}
	return v.data[htmlID]
}

// VFSRead GET /api/vfs/:htmlId/read?key=xxx
// 从虚拟文件系统读取数据
func VFSRead(c *gin.Context) {
	htmlID := parseHtmlID(c)
	if htmlID == 0 {
		utils.BadRequest(c, "无效的沙盒ID")
		return
	}
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "key不能为空")
		return
	}

	GlobalVFS.mu.RLock()
	bucket := GlobalVFS.data[htmlID]
	GlobalVFS.mu.RUnlock()

	if bucket == nil {
		utils.NotFound(c, "存储桶不存在")
		return
	}

	val, exists := bucket[key]
	if !exists {
		utils.NotFound(c, "键不存在")
		return
	}

	utils.Success(c, gin.H{
		"html_id": htmlID,
		"key":     key,
		"value":   val,
	})
}

// VFSWrite POST /api/vfs/:htmlId/write
// 写入数据到虚拟文件系统
func VFSWrite(c *gin.Context) {
	htmlID := parseHtmlID(c)
	if htmlID == 0 {
		utils.BadRequest(c, "无效的沙盒ID")
		return
	}

	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: key和value为必填")
		return
	}

	// 安全检查：禁止脚本类键名
	if isBlockedVFSKey(req.Key) {
		utils.BadRequest(c, "禁止使用脚本类键名: "+req.Key)
		return
	}

	// 大小限制：单值最大 1MB
	if len(req.Value) > 1024*1024 {
		utils.BadRequest(c, "单值不能超过 1MB")
		return
	}

	bucket := GlobalVFS.getBucket(htmlID)

	// 总量限制：每个沙盒最多 100 个key，总大小 10MB
	GlobalVFS.mu.RLock()
	totalSize := 0
	for _, v := range bucket {
		totalSize += len(v)
	}
	GlobalVFS.mu.RUnlock()

	if len(bucket) >= 100 && bucket[req.Key] == "" {
		utils.BadRequest(c, "存储容量已达上限（100个键）")
		return
	}
	if totalSize+len(req.Value) > 10*1024*1024 {
		utils.BadRequest(c, "存储总大小已达上限（10MB）")
		return
	}

	bucket[req.Key] = req.Value

	utils.Success(c, gin.H{
		"html_id": htmlID,
		"key":     req.Key,
		"written": len(req.Value),
	})
}

// VFSDelete DELETE /api/vfs/:htmlId/delete?key=xxx
// 从虚拟文件系统删除数据
func VFSDelete(c *gin.Context) {
	htmlID := parseHtmlID(c)
	if htmlID == 0 {
		utils.BadRequest(c, "无效的沙盒ID")
		return
	}
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "key不能为空")
		return
	}

	GlobalVFS.mu.Lock()
	bucket := GlobalVFS.data[htmlID]
	if bucket != nil {
		delete(bucket, key)
	}
	GlobalVFS.mu.Unlock()

	utils.Success(c, gin.H{"message": "已删除", "key": key})
}

// VFSList GET /api/vfs/:htmlId/list
// 列出沙盒的所有键
func VFSList(c *gin.Context) {
	htmlID := parseHtmlID(c)
	if htmlID == 0 {
		utils.BadRequest(c, "无效的沙盒ID")
		return
	}

	GlobalVFS.mu.RLock()
	bucket := GlobalVFS.data[htmlID]
	GlobalVFS.mu.RUnlock()

	var keys []gin.H
	totalSize := 0
	if bucket != nil {
		for k, v := range bucket {
			keys = append(keys, gin.H{"key": k, "size": len(v)})
			totalSize += len(v)
		}
	}
	if keys == nil {
		keys = []gin.H{}
	}

	utils.Success(c, gin.H{
		"html_id":    htmlID,
		"keys":       keys,
		"key_count":  len(keys),
		"total_size": totalSize,
		"max_keys":   100,
		"max_size":   "10MB",
	})
}

func parseHtmlID(c *gin.Context) uint {
	var id uint
	if _, err := fmt.Sscanf(c.Param("htmlId"), "%d", &id); err != nil {
		return 0
	}
	return id
}

func isBlockedVFSKey(key string) bool {
	// 禁止键名包含脚本扩展名
	for _, ext := range []string{".js", ".wasm", ".html", ".htm", ".sh", ".bat", ".exe", ".py", ".php"} {
		if len(key) >= len(ext) {
			if key[len(key)-len(ext):] == ext {
				return true
			}
		}
	}
	return false
}
