package service

import (
	"crypto/rand"
	"fmt"
	"time"
)

// newID 生成唯一 ID（时间戳 + 密码学随机段），供 service 层创建实体使用。
func newID(prefix string) string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%x", prefix, time.Now().UnixNano(), buf[:])
}
