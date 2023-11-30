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
		interval   time.Duration
		sourcePath string
		targets    []string
		checkPath  string
		regex      *regexp.Regexp
		status     map[int]bool
		mutex      sync.RWMutex
		logger     *log.Logger
	}

	Opts struct {
		Interval  time.Duration
		CheckPath string
		Regex     string
		Logger    *log.Logger
	}
)

var client = http.Client{Timeout: defaultTimeout}

func NewChecker(path string, targets []string, opts *Opts) *Checker {
	c := &Checker{
		sourcePath: path,
		targets:    targets,
		status:     make(map[int]bool),
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
		c.checkPath = opts.CheckPath
		if opts.Regex != "" {
			c.regex = regexp.MustCompile(opts.Regex)
		}
	}
	c.start()
	return c
}

func (c *Checker) start() {
	for i, target := range c.targets {
		if c.logger != nil {
			c.logger.Debug("health", "starting health check for path", c.sourcePath, target, "with interval", c.interval)
		}
		go c.healthCheckLoop(i, target)
	}
}

func (c *Checker) healthCheckLoop(index int, target string) {
	var healthFunc func(string) bool
	checkTarget, err := url.JoinPath(target, c.checkPath)
	if err != nil {
		panic(err)
	}
	if c.regex != nil {
		healthFunc = c.regexCheck
	} else {
		healthFunc = c.pingCheck
	}
	for {
		healthy := healthFunc(checkTarget)
		if c.logger != nil {
			c.logger.Debug("health", "path", c.sourcePath, "target", target, "healthy:", healthy)
		}
		c.mutex.Lock()
		c.status[index] = healthy
		c.mutex.Unlock()
		time.Sleep(c.interval)
	}
}

func (c *Checker) regexCheck(target string) bool {
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
	return c.regex.MatchString(string(bytes))
}

func (c *Checker) pingCheck(target string) bool {
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
