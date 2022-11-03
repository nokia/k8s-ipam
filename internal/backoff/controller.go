/*
Copyright 2022 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backoff

import (
	"context"
	"sync"
	"time"
)

type controller struct {
	ctx        context.Context
	cancel     func()
	maxRetries int
	mu         *sync.RWMutex
	next       chan struct{} // user-facing channel
	resetTimer chan time.Duration
	retries    int
	timer      *time.Timer
}

func newController(ctx context.Context) *controller {
	ctx, cancel := context.WithCancel(ctx)

	c := &controller{
		cancel:     cancel,
		ctx:        ctx,
		maxRetries: 5,
		mu:         &sync.RWMutex{},
		next:       make(chan struct{}, 1),
		resetTimer: make(chan time.Duration, 1),
		timer:      time.NewTimer(5 * time.Second),
	}

	// enqueue a single fake event so the user gets to retry once
	c.next <- struct{}{}

	go c.loop()
	return c
}

func (c *controller) loop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case d := <-c.resetTimer:
			if !c.timer.Stop() {
				select {
				case <-c.timer.C:
				default:
				}
			}
			c.timer.Reset(d)
		case <-c.timer.C:
			select {
			case <-c.ctx.Done():
				return
			case c.next <- struct{}{}:
			}
			if c.maxRetries > 0 {
				c.retries++
			}
			if !c.check() {
				c.cancel()
				return
			}
			c.resetTimer <- 5 * time.Second
		}
	}
}

func (c *controller) check() bool {
	return c.retries < c.maxRetries
}

func (c *controller) Done() <-chan struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ctx.Done()
}

func (c *controller) Next() <-chan struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.next
}
