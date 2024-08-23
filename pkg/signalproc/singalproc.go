package signalproc

import (
	"gnano/internal/log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
)

func signalReload() {
	log.Println("viper reload config start")
	viper.ReadInConfig()
	log.Println("viper reload config end")
}

func signalQuit() {
	// os.Exit(0)
	log.Println("signalproc.signalQuit")
}

// 如果自定义函数reload和quit函数, 因为涉及多协程交互, 注意用管道传递动作信号
// 启动信号处理函数
func Startup(reload, quit func()) {
	go func() {
		ch := make(chan os.Signal, 1)
		if reload == nil {
			reload = signalReload
		}

		if quit == nil {
			quit = signalQuit
		}
		//拦截hup信号和term信号，分别用于重载和退出
		//使用这两个信号的原因是各个操作系统都有实现
		signal.Notify(ch, syscall.SIGHUP, syscall.SIGTERM)
		log.Println("signalproc.Startup signal.Notify(ch, syscall.SIGHUP, syscall.SIGTERM)")
		for {
			s := <-ch
			switch s {
			case syscall.SIGHUP:
				//触发重载信号
				reload()
			case syscall.SIGTERM:
				//触发退出信号
				quit()
			}
		}
	}()
}
