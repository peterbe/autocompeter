package main

import (
	"encoding/json"
	// "fmt"
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

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.Put(c)
	SetDomain("xyz1234567890", "peterbe.com", c)

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
	form := url.Values{}
	form.Add("title", "Some blog title")
	form.Add("url", "/some/page")
	request, err := http.NewRequest("POST", "/v1", strings.NewReader(form.Encode()))
	if err != nil {
		panic(err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Auth-Key", "xyz1234567890")
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
	form.Add("title", "Some blog title")
	form.Add("url", "/some/page")
	request, err := http.NewRequest("POST", "/v1", strings.NewReader(form.Encode()))
	if err != nil {
		panic(err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Auth-Key", "xyz1234567890")
	response := httptest.NewRecorder()

	updateHandler(response, request)
	assert.Equal(suite.T(), response.Code, http.StatusCreated)

	// now delete it
	form = url.Values{}
	// form.Add("domain", "peterbe.com")
	form.Add("url", "/some/page")
	request, err = http.NewRequest("DELETE", "/v1?"+form.Encode(), nil)
	request.Header.Add("Auth-Key", "xyz1234567890")
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

func (suite *HandlerSuite) TestPingHandler() {
	request, _ := http.NewRequest("GET", "/v1/ping", nil)
	response := httptest.NewRecorder()

	pingHandler(response, request)
	assert.Equal(suite.T(), response.Body.String(), "pong\n")
	assert.Equal(suite.T(), response.Code, http.StatusOK)
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
	var expanded bool
	got, expanded = cleanWords("two words")
	expect = []string{"two", "words"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, false)
}

func TestCleanWordsNumbers(t *testing.T) {
	var got, expect []string
	var expanded bool
	got, expanded = cleanWords("Monday 2015")
	expect = []string{"monday", "2015"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, false)
	// independent of order
	got, expanded = cleanWords("2014 Tuesday")
	expect = []string{"2014", "tuesday"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, false)
}

func TestCleanWordsNormalized(t *testing.T) {
	var got, expect []string
	var expanded bool
	got, expanded = cleanWords("TWO   wOrds")
	expect = []string{"two", "words"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, false)
}

func TestCleanWordsCleaned(t *testing.T) {
	var got, expect []string
	var expanded bool
	got, expanded = cleanWords(" word\"[](){}?!.-:;,' ")
	expect = []string{"word"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, false)

	got, expanded = cleanWords("sentence.")
	expect = []string{"sentence"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, false)
}

func TestCleanWordsUnidecoded(t *testing.T) {
	var got, expect []string
	var expanded bool
	got, expanded = cleanWords(" trött ")
	expect = []string{"trött", "trott"}
	assert.Equal(t, got, expect)
	assert.Equal(t, expanded, true)
}

func TestEncodeString(t *testing.T) {
	got := encodeString("www.peterbe.com")
	// first 6 characters of that domain base64 encoded of its md5 checksum
	expect := "6590cz"
	assert.Equal(t, got, expect)
}

func TestGetPrefixes(t *testing.T) {
	got := getPrefixes("word")
	expect := []string{"w", "wo", "wor", "word", "word$"}
	assert.Equal(t, got, expect)
}

func TestGetPrefixesWithUnicode(t *testing.T) {
	got := getPrefixes("würd")
	expect := []string{
		"w", "wü", "wür", "würd", "würd$",
		"w", "wu", "wur", "wurd", "wurd$",
	}
	assert.Equal(t, got, expect)
}
