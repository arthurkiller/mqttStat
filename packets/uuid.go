package packets

import (
	"strconv"
	"sync/atomic"
	"time"
)

type UUID string

var number int64 = 0

func NewUUID() UUID {
	return UUID(
		strconv.FormatInt(time.Now().Unix(), 10) +
			"-" +
			strconv.FormatInt(atomic.AddInt64(&number, 1), 10))
}

func (u UUID) String() string {
	return string(u)
}
