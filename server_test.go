package main

import (
	"encoding/json"
	"fmt"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Set up the suite for running tests with Redis
type HandlerSuite struct {
	suite.Suite
}

func (suite *HandlerSuite) SetupTest() {
	fmt.Println("In SetupTest()")
	var err error
	// var redis_url =
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

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

type OKMessage struct {
	Message string
}

type Response struct {
	Terms   []string
	Results [][]string
}

func (suite *HandlerSuite) TestUpdateAndFetch() {
	// request, _ := http.NewRequest("POST", "/v1", nil)
	form := url.Values{}
	form.Add("domain", "peterbe.com")
	form.Add("title", "Some blog title")
	form.Add("url", "/some/page")
	request, err := http.NewRequest("POST", "/v1", strings.NewReader(form.Encode()))
	if err != nil {
		panic(err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	updateHandler(response, request)
	decoder := json.NewDecoder(response.Body)
	var t OKMessage
	err = decoder.Decode(&t)
	if err != nil {
		panic(err)
	}
	assert.Equal(suite.T(), t.Message, "OK")
	assert.Equal(suite.T(), response.Code, http.StatusCreated)

	// now fetch from it
	request, _ = http.NewRequest("GET", "/v1?d=peterbe.com&q=blo", nil)
	response = httptest.NewRecorder()
	fetchHandler(response, request)
	assert.Equal(suite.T(), response.Code, http.StatusOK)

	decoder = json.NewDecoder(response.Body)
	var r Response
	err = decoder.Decode(&r)
	if err != nil {
		panic(err)
	}
	assert.Equal(suite.T(), r.Terms, []string{"blo"})
	assert.Equal(suite.T(), len(r.Results), 1)
	assert.Equal(suite.T(), r.Results[0][0], "/some/page")
	assert.Equal(suite.T(), r.Results[0][1], "Some blog title")

}

func (suite *HandlerSuite) TestUpdateAndDelete() {
	form := url.Values{}
	form.Add("domain", "peterbe.com")
	form.Add("title", "Some blog title")
	form.Add("url", "/some/page")
	request, err := http.NewRequest("POST", "/v1", strings.NewReader(form.Encode()))
	if err != nil {
		panic(err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	updateHandler(response, request)
	assert.Equal(suite.T(), response.Code, http.StatusCreated)

	// now delete it
	form = url.Values{}
	form.Add("domain", "peterbe.com")
	form.Add("url", "/some/page")
	request, err = http.NewRequest("DELETE", "/v1?"+form.Encode(), nil)
	if err != nil {
		panic(err)
	}
	response = httptest.NewRecorder()

	deleteHandler(response, request)
	assert.Equal(suite.T(), response.Code, http.StatusNoContent)

	// now fetch from it
	request, _ = http.NewRequest("GET", "/v1?d=peterbe.com&q=blo", nil)
	response = httptest.NewRecorder()
	fetchHandler(response, request)
	assert.Equal(suite.T(), response.Code, http.StatusOK)

	decoder := json.NewDecoder(response.Body)
	var r Response
	err = decoder.Decode(&r)
	if err != nil {
		panic(err)
	}
	assert.Equal(suite.T(), r.Terms, []string{"blo"})
	assert.Equal(suite.T(), len(r.Results), 0)
}

// Test the IndexHandler without Redis
func TestIndexHandlerReturnsWithStatusOK(t *testing.T) {
	request, _ := http.NewRequest("GET", "/", nil)
	response := httptest.NewRecorder()

	indexHandler(response, request)
	// fmt.Println(response.Body.String())
	if response.Code != http.StatusOK {
		t.Fatalf("Non-expected status code%v:\n\tbody: %v", "200", response.Code)
	}
}

func TestCleanWordsBasic(t *testing.T) {
	var got, expect []string
	got = cleanWords("two words")
	expect = []string{"two", "words"}
	assert.Equal(t, got, expect)
}

func TestCleanWordsNormalized(t *testing.T) {
	var got, expect []string
	got = cleanWords("TWO   wOrds")
	expect = []string{"two", "words"}
	assert.Equal(t, got, expect)
}

func TestCleanWordsCleaned(t *testing.T) {
	var got, expect []string
	got = cleanWords(" word\"[](){}?!,-:;,' ")
	expect = []string{"word"}
	assert.Equal(t, got, expect)

	got = cleanWords("sentence.")
	expect = []string{"sentence"}
	assert.Equal(t, got, expect)
}

func TestCleanWordsUnidecoded(t *testing.T) {
	var got, expect []string
	got = cleanWords(" trött ")
	expect = []string{"trött", "trott"}
	assert.Equal(t, got, expect)
}

func TestEncodeString(t *testing.T) {
	got := encodeString("www.peterbe.com")
	expect := "eb9f7473" // first 8 characters
	assert.Equal(t, got, expect)
}

func TestGetPrefixes(t *testing.T) {
	got := getPrefixes("word")
	expect := []string{"w", "wo", "wor", "word"}
	assert.Equal(t, got, expect)
}
