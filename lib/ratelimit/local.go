// Copyright (c) 2018 Ashley Jeffs
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Jeffail/benthos/lib/log"
	"github.com/Jeffail/benthos/lib/metrics"
	"github.com/Jeffail/benthos/lib/types"
)

//------------------------------------------------------------------------------

func init() {
	Constructors[TypeLocal] = TypeSpec{
		constructor: NewLocal,
		description: `
The local rate limit is a simple X every Y type rate limit that can be shared
across any number of components within the pipeline.`,
	}
}

//------------------------------------------------------------------------------

// LocalConfig is a config struct containing rate limit fields for a local rate
// limit.
type LocalConfig struct {
	Count    int    `json:"count" yaml:"count"`
	Interval string `json:"interval" yaml:"interval"`
}

// NewLocalConfig returns a local rate limit configuration struct with default
// values.
func NewLocalConfig() LocalConfig {
	return LocalConfig{
		Count:    1000,
		Interval: "1s",
	}
}

//------------------------------------------------------------------------------

// Local is a structure that tracks a rate limit, it can be shared across
// parallel processes in order to maintain a maximum rate of a protected
// resource.
type Local struct {
	mut         sync.Mutex
	bucket      int
	lastRefresh time.Time

	size   int
	period time.Duration
}

// NewLocal creates a local rate limit from a configuration struct. This type is
// safe to share and call from parallel goroutines.
func NewLocal(
	conf Config,
	mgr types.Manager,
	logger log.Modular,
	stats metrics.Type,
) (types.RateLimit, error) {
	if conf.Local.Count <= 0 {
		return nil, errors.New("count must be larger than zero")
	}
	period, err := time.ParseDuration(conf.Local.Interval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interval: %v", err)
	}
	return &Local{
		bucket:      conf.Local.Count,
		lastRefresh: time.Now(),
		size:        conf.Local.Count,
		period:      period,
	}, nil
}

//------------------------------------------------------------------------------

// Access the rate limited resource. Returns a duration or an error if the rate
// limit check fails. The returned duration is either zero (meaning the resource
// can be accessed) or a reasonable length of time to wait before requesting
// again.
func (r *Local) Access() (time.Duration, error) {
	r.mut.Lock()
	r.bucket--

	if r.bucket < 0 {
		r.bucket = 0
		remaining := r.period - time.Since(r.lastRefresh)

		if remaining > 0 {
			r.mut.Unlock()
			return remaining, nil
		}
		r.bucket = r.size - 1
		r.lastRefresh = time.Now()
	}
	r.mut.Unlock()
	return 0, nil
}

// CloseAsync shuts down the rate limit.
func (r *Local) CloseAsync() {
}

// WaitForClose blocks until the rate limit has closed down.
func (r *Local) WaitForClose(timeout time.Duration) error {
	return nil
}

//------------------------------------------------------------------------------
