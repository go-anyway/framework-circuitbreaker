// Copyright 2025 zampo.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// @contact  zampo3380@gmail.com

package circuitbreaker

import (
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreaker 熔断器包装
type CircuitBreaker struct {
	cb       *gobreaker.CircuitBreaker
	name     string
	settings Settings
	mu       sync.RWMutex
}

// Settings 熔断器配置
type Settings struct {
	// MaxRequests 半开状态下允许的最大请求数
	MaxRequests uint32
	// Interval 关闭状态下的时间窗口（秒）
	Interval time.Duration
	// Timeout 打开状态下的超时时间（秒），之后尝试半开
	Timeout time.Duration
	// ReadyToTrip 自定义的熔断触发函数
	ReadyToTrip func(counts gobreaker.Counts) bool
}

// DefaultSettings 返回默认配置
func DefaultSettings() Settings {
	return Settings{
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: nil, // 使用默认：连续失败5次触发熔断
	}
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(name string, settings Settings) *CircuitBreaker {
	cbSettings := gobreaker.Settings{
		Name:        name,
		MaxRequests: settings.MaxRequests,
		Interval:    settings.Interval,
		Timeout:     settings.Timeout,
	}

	if settings.ReadyToTrip != nil {
		cbSettings.ReadyToTrip = settings.ReadyToTrip
	}

	return &CircuitBreaker{
		cb:       gobreaker.NewCircuitBreaker(cbSettings),
		name:     name,
		settings: settings,
	}
}

// Execute 执行函数，带熔断保护
func (cb *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.cb.Execute(fn)
}

// State 获取当前熔断器状态
func (cb *CircuitBreaker) State() gobreaker.State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.cb.State()
}

// Counts 获取统计信息
func (cb *CircuitBreaker) Counts() gobreaker.Counts {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.cb.Counts()
}

// UpdateSettings 更新熔断器配置（热更新）
// 注意：这会重新创建内部的熔断器实例，会丢失当前状态
func (cb *CircuitBreaker) UpdateSettings(settings Settings) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cbSettings := gobreaker.Settings{
		Name:        cb.name,
		MaxRequests: settings.MaxRequests,
		Interval:    settings.Interval,
		Timeout:     settings.Timeout,
	}

	if settings.ReadyToTrip != nil {
		cbSettings.ReadyToTrip = settings.ReadyToTrip
	}

	// 创建新的熔断器实例
	cb.cb = gobreaker.NewCircuitBreaker(cbSettings)
	cb.settings = settings
}

// GetSettings 获取当前配置
func (cb *CircuitBreaker) GetSettings() Settings {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.settings
}
