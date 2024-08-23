package async

import (
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
)

func pcall(fn func(), zLog *zap.Logger) {
	defer func() {
		if err := recover(); err != nil {
			zLog.Error(fmt.Sprintf("pcall panic: %+v\n%s", err, debug.Stack()))
		}
	}()
	fn()
}
func Run(fn func(), zLog *zap.Logger) {
	go pcall(fn, zLog)
}
