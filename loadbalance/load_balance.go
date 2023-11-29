package loadbalance

import (
	"net/http"
	"sync"

	"github.com/doggystylez/proxy"
	"github.com/doggystylez/proxy/health"
)

type (
	roundrobinMap map[string]proxy.Backend

	roundrobin struct {
		counter int
		path    string
		targets []*proxy.Proxy
		health  *health.Checker
		mutex   sync.Mutex
	}
)

func NewRoundRobin(mapping map[string][]string, opts *health.Opts) (roundrobinMap, error) {
	p := make(roundrobinMap, len(mapping))
	for path, urls := range mapping {
		rr := &roundrobin{path: path, health: health.NewChecker(path, urls, opts)}
		for i, url := range urls {
			u, err := proxy.ValidateUrl(url)
			if err != nil {
				return nil, err
			}
			prox := proxy.NewProxy(u, i)
			rr.targets = append(rr.targets, prox)
		}
		p[path] = rr
	}
	return p, nil
}

func (rr *roundrobin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rr.mutex.Lock()
	defer func() {
		rr.mutex.Unlock()
		rr.counter = (rr.counter + 1) % len(rr.targets)
	}()
	rr.nextHealthyProxy().ServeHTTP(w, r)
}

func (rr *roundrobin) nextHealthyProxy() *proxy.Proxy {
	checked := 0
	for checked < len(rr.targets) {
		if rr.health.Status(rr.targets[rr.counter].Index()) {
			return rr.targets[rr.counter]
		}
		rr.counter = (rr.counter + 1) % len(rr.targets)
		checked++
	}
	return nil
}
