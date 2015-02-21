package main

import (
	// "fmt"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

// Set up the suite for running tests with Redis
type Suite struct {
	suite.Suite
}

func (suite *Suite) SetupTest() {
	var err error
	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		// fmt.Println("DIaling")
		if err != nil {
			return nil, err
		}
		err = client.Cmd("SELECT", 8).Err
		if err != nil {
			return nil, err
		}
		err = client.Cmd("FLUSHDB").Err
		if err != nil {
			return nil, err
		}
		return client, nil
	}
	redisPool, err = pool.NewCustomPool("tcp", redisURL, 1, df)
	if err != nil {
		panic(err)
	}
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestGetDomainSomething() {
	// authKeys = new(AuthKeys)
	// authKeys.Init()

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.Put(c)
	err = c.Cmd("HSET", "$domainkeys", "xyz1234567890", "peterbe.com").Err
	errHndlr(err)

	domain, err := GetDomain("xyz1234567890", c)
	errHndlr(err)
	assert.Equal(
		suite.T(),
		domain,
		"peterbe.com",
	)
}

func (suite *Suite) TestGetDomainNothing() {
	// authKeys = new(AuthKeys)
	// authKeys.Init()

	c, err := redisPool.Get()
	errHndlr(err)

	domain, err := GetDomain("xyz1234567890", c)
	errHndlr(err)
	assert.Equal(
		suite.T(),
		domain,
		"",
	)
}
