package main

import "github.com/fzzy/radix/redis"

type AuthKeys struct {
	Domains map[string]string
}

func (ak *AuthKeys) Init() {
	ak.Domains = make(map[string]string)
}
func (ak *AuthKeys) GetDomain(key string, conn *redis.Client) (string, error) {
	if domain, ok := ak.Domains[key]; ok {
		return domain, nil
	} else {
		domain, err := conn.Cmd("HGET", "$domainkeys", key).Str()
		if err != nil {
			return "", err
		}
		// now cache the value
		// perhaps we should remember this with a timestamp
		ak.Domains[key] = domain
		return domain, nil
	}
}
