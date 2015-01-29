package main

import (
	// "net/http"
	// "net/http/httptest"
	"github.com/stretchr/testify/assert"
	"testing"
	// "fmt"
)

// func TestHandleIndexReturnsWithStatusOK(t *testing.T) {
// 	request, _ := http.NewRequest("GET", "/", nil)
// 	response := httptest.NewRecorder()
//
// 	IndexHandler(response, request)
//
// 	if response.Code != http.StatusOK {
// 		t.Fatalf("Non-expected status code%v:\n\tbody: %v", "200", response.Code)
// 	}
// }

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
