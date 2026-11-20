// Package maintenance 是热迁移期间的全局写入闸门。
//
// 迁移一旦开始就不能再让正文/上传/点赞等写入落到源库，否则目标库会缺数据；
// 但登录与管理端迁移状态查询必须继续可用，否则用户连「正在维护」的提示都看不到。
package maintenance

import "sync/atomic"

var enabled atomic.Bool

func Enabled() bool { return enabled.Load() }
func Begin()        { enabled.Store(true) }
func End()          { enabled.Store(false) }
