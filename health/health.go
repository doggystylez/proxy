package health

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/doggystylez/utils/log"
)

const (
	defaultInterval = 5 * time.Minute
	defaultTimeout  = 10 * time.Second
)

type (
	Checker struct {
		interval time.Duration
		path     string
		targets  []string
		status   map[int]bool
		mutex    sync.RWMutex
		logger   *log.Logger
		*CustomCheck
	}

	CustomCheck struct {
		path  string
		check *regexp.Regexp
	}

	Opts struct {
		Interval    time.Duration
		CustomCheck *CustomCheck
		Logger      *log.Logger
	}
)

var client = http.Client{Timeout: defaultTimeout}

func NewChecker(path string, targets []string, opts *Opts) *Checker {
	c := &Checker{
		path:    path,
		targets: targets,
		status:  make(map[int]bool),
	}
	if opts == nil {
		c.interval = defaultInterval
		client.Timeout = defaultTimeout
	} else {
		if opts.Interval == 0 {
			c.interval = defaultInterval
		} else {
			c.interval = opts.Interval
		}
		c.logger = opts.Logger
		c.CustomCheck = opts.CustomCheck
	}
	c.start()
	return c
}

func NewCustomCheck(path, check string) *CustomCheck {
	return &CustomCheck{path: path, check: regexp.MustCompile(regexp.QuoteMeta(check))}
}

func (c *Checker) start() {
	for i, target := range c.targets {
		if c.logger != nil {
			c.logger.Debug("health", "starting health check for", "path", c.path, target, "with interval", c.interval)
		}
		go c.healthCheckLoop(i, target)
	}
}

func (c *Checker) healthCheckLoop(index int, target string) {
	var (
		healthy    bool
		healthFunc func(string) bool
		err        error
	)
	if c.CustomCheck != nil {
		target, err = url.JoinPath(target, c.CustomCheck.path)
		if err != nil {
			panic(err)
		}
		healthFunc = c.custom
	} else {
		healthFunc = c.ping
	}
	for {
		healthy = healthFunc(target)
		if c.logger != nil {
			c.logger.Debug("health", "path", c.path, "target", target, "healthy:", healthy)
		}
		c.mutex.Lock()
		c.status[index] = healthy
		c.mutex.Unlock()
		time.Sleep(c.interval)
	}
}

func (c *Checker) custom(target string) bool {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		panic(err)
	}
	response, err := client.Do(request)
	if err != nil {
		if c.logger != nil {
			c.logger.Debug("health", request.URL, "error:", err)
		}
		return false
	}
	defer response.Body.Close() //nolint: errcheck
	if response.StatusCode != http.StatusOK {
		if c.logger != nil {
			c.logger.Debug("health", request.URL, "status code:", response.StatusCode)
		}
		return false
	}
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		if c.logger != nil {
			c.logger.Debug("health", request.URL, "error:", err)
		}
		return false
	}
	return c.check.MatchString(string(bytes))
}

func (c *Checker) ping(target string) bool {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		panic(err)
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close() //nolint: errcheck
	return response.StatusCode == http.StatusOK
}

func (c *Checker) Status(index int) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.status[index]
}
