package ui

import (
	"log"
	"time"

	"fyne.io/fyne/v2"
	"go.lucor.dev/paw/internal/model"
)

type autoLockOption struct {
	key, label string
	delay      time.Duration
}

var autoLockChoices = []autoLockOption{
	{model.AutoLockNever, "Never", 0},
	{model.AutoLockOneMinute, "1 minute", time.Minute},
	{model.AutoLockFiveMinutes, "5 minutes", 5 * time.Minute},
	{model.AutoLockTenMinutes, "10 minutes", 10 * time.Minute},
	{model.AutoLockFifteenMinutes, "15 minutes", 15 * time.Minute},
	{model.AutoLockThirtyMinutes, "30 minutes", 30 * time.Minute},
	{model.AutoLockOneHour, "1 hour", time.Hour},
	{model.AutoLockTwoHours, "2 hours", 2 * time.Hour},
	{model.AutoLockFourHours, "4 hours", 4 * time.Hour},
	{model.AutoLockEightHours, "8 hours", 8 * time.Hour},
	{model.AutoLockOneDay, "1 day", 24 * time.Hour},
}

func autoLockLabels() []string {
	labels := make([]string, len(autoLockChoices))
	for i, option := range autoLockChoices {
		labels[i] = option.label
	}
	return labels
}
func autoLockLabel(key string) string {
	for _, option := range autoLockChoices {
		if option.key == key {
			return option.label
		}
	}
	return settingsAutoLockNever
}
func autoLockKey(label string) string {
	for _, option := range autoLockChoices {
		if option.label == label {
			return option.key
		}
	}
	return model.AutoLockNever
}
func autoLockByKey(key string) (time.Duration, bool) {
	for _, option := range autoLockChoices {
		if option.key == key {
			return option.delay, option.delay > 0
		}
	}
	return 0, false
}

// backgroundAutoLock is accessed only on the Fyne UI thread.
type backgroundAutoLock struct {
	now        func() time.Time
	lock       func()
	timer      *time.Timer
	started    time.Time
	delay      time.Duration
	generation uint64
}

func newBackgroundAutoLock(lock func()) *backgroundAutoLock {
	return &backgroundAutoLock{now: time.Now, lock: lock}
}
func (a *backgroundAutoLock) stop() {
	a.generation++
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
}
func (a *backgroundAutoLock) configure(key string) {
	a.stop()
	a.delay, _ = autoLockByKey(key)
	log.Printf("auto-lock: configured key=%q timeout=%s", key, a.delay)
	if a.delay == 0 && key != "" && key != model.AutoLockNever {
		log.Printf("auto-lock: unsupported timeout key=%q; disabled", key)
	}
	a.schedule()
}
func (a *backgroundAutoLock) exitedForeground() {
	if !a.started.IsZero() {
		return
	}
	a.started = a.now()
	log.Printf("auto-lock: entered background timeout=%s", a.delay)
	a.schedule()
}
func (a *backgroundAutoLock) enteredForeground() {
	log.Print("auto-lock: entered foreground")
	expired := a.expired()
	a.stop()
	a.started = time.Time{}
	if expired {
		a.lock()
	}
}
func (a *backgroundAutoLock) expired() bool {
	return a.delay > 0 && !a.started.IsZero() && !a.now().Before(a.started.Add(a.delay))
}
func (a *backgroundAutoLock) schedule() {
	if a.delay <= 0 || a.started.IsZero() {
		return
	}
	if a.expired() {
		a.expire(a.generation)
		return
	}
	remaining := a.started.Add(a.delay).Sub(a.now())
	generation := a.generation
	a.timer = time.AfterFunc(remaining, func() {
		fyne.Do(func() { a.expire(generation) })
	})
	log.Printf("auto-lock: timer scheduled remaining=%s", remaining)
}
func (a *backgroundAutoLock) expire(generation uint64) {
	if generation != a.generation || !a.expired() {
		return
	}
	a.stop()
	log.Print("auto-lock: timeout expired")
	a.lock()
}
func (a *backgroundAutoLock) close() {
	a.stop()
	a.started = time.Time{}
}
