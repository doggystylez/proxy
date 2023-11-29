package proxy

import (
	"net/url"
)

func NewSimpleProxy(mapping Mapping) (map[string]Backend, error) {
	p := make(map[string]Backend, len(mapping))
	for path, targets := range mapping {
		targetURL, err := ValidateUrl(targets[0])
		if err != nil {
			return nil, err
		}
		proxy := NewProxy(targetURL, 0)
		p[path] = proxy
	}
	return p, nil
}

func ValidateUrl(target string) (*url.URL, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, &ErrInvalidTarget{target: target}
	}
	return u, nil
}
