// Copyright (c) nano Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package scheduler

import (
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"gnano/internal/env"
	"gnano/internal/log"
)

const (
	messageQueueBacklog = 1 << 10
	sessionCloseBacklog = 1 << 8

	// 关闭
	statusClosed = 4
)

// LocalScheduler schedules task to a customized goroutine
type LocalScheduler interface {
	Schedule(Task)
	IsClose() bool
}

type Task func()

type Hook func()

// 按会话进行绑定
type QueueLocalScheduler struct {
	chTasks   chan Task
	chDie     chan struct{}
	status    int32
	openTime  time.Time
	closeTime time.Time
}

// QueueLocalScheduler
func NewQueueLocalScheduler() *QueueLocalScheduler {
	qs := &QueueLocalScheduler{chTasks: make(chan Task, 1<<6), chDie: make(chan struct{}), openTime: time.Now()}
	log.Println("open time %v", qs.openTime.String())
	//消费消息队列
	go qs.Sched()
	return qs
}

// Schedule 读协程往自定义消息队列写入任务
func (queueScheduler *QueueLocalScheduler) Schedule(task Task) {
	if queueScheduler.IsClose() {
		log.Println("localScheduler  statusClosed ", queueScheduler.closeTime.String())
		return
	}

	select {
	case queueScheduler.chTasks <- task:
	case <-time.After(5 * time.Second):
		log.Println("localScheduler.PushTask  Timeout ")

	}
}

func (queueScheduler *QueueLocalScheduler) IsClose() bool {
	return atomic.LoadInt32(&queueScheduler.status) == statusClosed
}

// Sched 消息自定义协程任务
func (localScheduler *QueueLocalScheduler) Sched() {
	defer func() {
		atomic.StoreInt32(&localScheduler.status, statusClosed)
		localScheduler.closeTime = time.Now()
		close(localScheduler.chTasks)
	}()

	for {
		select {
		case f := <-localScheduler.chTasks:
			try(f)
		case <-localScheduler.chDie:
			log.Println("localScheduler chDie close")
			return
		}
	}
}

func (localScheduler *QueueLocalScheduler) Close() {
	if atomic.LoadInt32(&localScheduler.status) == statusClosed {
		return
	}
	close(localScheduler.chDie)
}

// 按会话进行绑定
type TimerQueueScheduler struct {
	*time.Ticker
	chTasks   chan Task
	chDie     chan struct{}
	status    int32
	hooks     []Hook
	openTime  time.Time
	closeTime time.Time
}

// QueueLocalScheduler
func NewTimerQueueScheduler(d time.Duration) *TimerQueueScheduler {
	qs := &TimerQueueScheduler{chTasks: make(chan Task, 1<<6), chDie: make(chan struct{}), openTime: time.Now(), Ticker: time.NewTicker(d), hooks: make([]Hook, 0)}
	log.Println("open time %v", qs.openTime.String())
	//消费消息队列
	go qs.Sched()
	return qs
}

// Schedule 读协程往自定义消息队列写入任务
func (tqs *TimerQueueScheduler) Schedule(task Task) {
	if tqs.IsClose() {
		log.Println("localScheduler  statusClosed  close time ", tqs.closeTime.String())
		return
	}
	select {
	case tqs.chTasks <- task:
	case <-time.After(5 * time.Second):
		log.Println("localScheduler.PushTask  Timeout ")
	}
}

func (tqs *TimerQueueScheduler) OnHook(hk Hook) {
	tqs.hooks = append(tqs.hooks, hk)
}

func (tqs *TimerQueueScheduler) RemoveHook() {
	tqs.hooks = make([]Hook, 0)
}

func (tqs *TimerQueueScheduler) IsClose() bool {
	return atomic.LoadInt32(&tqs.status) == statusClosed
}

// Sched 消息自定义协程任务
func (tqs *TimerQueueScheduler) Sched() {
	defer func() {
		atomic.StoreInt32(&tqs.status, statusClosed)
		tqs.closeTime = time.Now()
		tqs.hooks = make([]Hook, 0)
		close(tqs.chTasks)
		tqs.Stop()
	}()

	for {
		select {
		case <-tqs.C:
			for _, hk := range tqs.hooks {
				try(hk)
			}
		case f := <-tqs.chTasks:
			try(f)
		case <-tqs.chDie:
			log.Println("localScheduler chDie close")
			return
		}
	}
}

func (tqs *TimerQueueScheduler) Close() {
	if atomic.LoadInt32(&tqs.status) == statusClosed {
		return
	}
	close(tqs.chDie)
}

// 每个读线程 会同时进行访问 存在并发问题
type DefaultScheduler struct{}

func NewDefaultScheduler() *DefaultScheduler {
	return &DefaultScheduler{}
}

// Schedule 每个读线程直接执行
func (defaultScheduler *DefaultScheduler) Schedule(task Task) {
	try(task)
}

var (
	chDie   = make(chan struct{})
	chExit  = make(chan struct{})
	chTasks = make(chan Task, 1<<8)
	started int32
	closed  int32
)

func try(f func()) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(fmt.Sprintf("Handle message panic: %+v\n%s", err, debug.Stack()))
		}
	}()
	f()
}

func Sched() {
	if atomic.AddInt32(&started, 1) != 1 {
		return
	}

	ticker := time.NewTicker(env.TimerPrecision)
	defer func() {
		ticker.Stop()
		close(chExit)
	}()

	for {
		select {
		case <-ticker.C:
			cron()
		case f := <-chTasks:
			try(f)
		case <-chDie:
			return
		}
	}
}

func Close() {
	if atomic.AddInt32(&closed, 1) != 1 {
		return
	}
	close(chDie)
	<-chExit
	log.Println("Scheduler stopped")
}

func PushTask(task Task) {
	select {
	case chTasks <- task:
	case <-time.After(5 * time.Second):
		log.Println("PushTask  Timeout ")
	}
	// log.Println("push task success")
}
