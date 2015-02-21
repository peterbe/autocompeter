package main

import "github.com/fzzy/radix/redis"

func GetDomain(key string, conn *redis.Client) (string, error) {
	reply := conn.Cmd("HGET", "$domainkeys", key)
	if reply.Type == redis.NilReply {
		return "", reply.Err
	}
	domain, err := reply.Str()
	if err != nil {
		return "", err
	} else {
		return domain, nil
	}
}

func SetDomain(key string, domain string, conn *redis.Client) error {
	return conn.Cmd("HSET", "$domainkeys", key, domain).Err
}
