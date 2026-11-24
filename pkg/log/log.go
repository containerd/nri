/*
   Copyright The containerd Authors.

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

package log

import (
	"context"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// log is the logger in use. It is stored atomically: a runtime can set
// a logger at any point, including while NRI is running and logging
// from goroutines of its own.
var log atomic.Pointer[Logger]

// Logger is the interface NRI uses for logging.
type Logger interface {
	Debugf(ctx context.Context, format string, args ...any)
	Infof(ctx context.Context, format string, args ...any)
	Warnf(ctx context.Context, format string, args ...any)
	Errorf(ctx context.Context, format string, args ...any)
}

// Set the logger used by NRI.
func Set(l Logger) {
	log.Store(&l)
}

// Get the logger used by NRI.
func Get() Logger {
	if l := log.Load(); l != nil {
		return *l
	}
	return fallback
}

var fallback Logger = &fallbackLogger{}

// Debugf logs a formatted debug message.
func Debugf(ctx context.Context, format string, args ...any) {
	Get().Debugf(ctx, format, args...)
}

// Infof logs a formatted informational message.
func Infof(ctx context.Context, format string, args ...any) {
	Get().Infof(ctx, format, args...)
}

// Warnf logs a formatted warning message.
func Warnf(ctx context.Context, format string, args ...any) {
	Get().Warnf(ctx, format, args...)
}

// Errorf logs a formatted error message.
func Errorf(ctx context.Context, format string, args ...any) {
	Get().Errorf(ctx, format, args...)
}

type fallbackLogger struct{}

// Debugf logs a formatted debug message.
func (f *fallbackLogger) Debugf(ctx context.Context, format string, args ...any) {
	logrus.WithContext(ctx).Debugf(format, args...)
}

// Infof logs a formatted informational message.
func (f *fallbackLogger) Infof(ctx context.Context, format string, args ...any) {
	logrus.WithContext(ctx).Infof(format, args...)
}

// Warnf logs a formatted warning message.
func (f *fallbackLogger) Warnf(ctx context.Context, format string, args ...any) {
	logrus.WithContext(ctx).Warnf(format, args...)
}

// Errorf logs a formatted error message.
func (f *fallbackLogger) Errorf(ctx context.Context, format string, args ...any) {
	logrus.WithContext(ctx).Errorf(format, args...)
}
