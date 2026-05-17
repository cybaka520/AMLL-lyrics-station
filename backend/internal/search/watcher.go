package search

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type WatchEvent struct {
	Source string
	Name   string
}

func Watch(ctx context.Context, paths []string, onChange func(evt WatchEvent) error) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := w.Add(p); err != nil {
			_ = w.Close()
			return err
		}
	}
	go func() {
		defer w.Close()
		const debounce = 1500 * time.Millisecond
		var timer *time.Timer
		var last WatchEvent
		trigger := func(evt WatchEvent) {
			last = evt
			if timer == nil {
				timer = time.NewTimer(debounce)
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok { return }
				if strings.HasSuffix(ev.Name, ".ttml") || filepath.Base(ev.Name) == filepath.Base(paths[0]) {
					trigger(WatchEvent{Source: ev.Op.String(), Name: ev.Name})
				}
			case err, ok := <-w.Errors:
				if !ok { return }
				_ = err
			case <-func() <-chan time.Time { if timer == nil { return make(chan time.Time) }; return timer.C }():
				_ = onChange(last)
			}
		}
	}()
	return nil
}
