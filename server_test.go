package main

import (
	// "net/http"
	// "net/http/httptest"
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

func equalArrays(arr1, arr2 []string) bool {
	if len(arr1) != len(arr2) {
		return false
	}
	for i, element := range arr1 {
		if element != arr2[i] {
			return false
		}
	}
	return true
}

func TestCleanWordsBasic(t *testing.T) {
	var got, expect []string
	got = cleanWords("two words")
	expect = []string{"two", "words"}
	if !equalArrays(got, expect) {
		t.Errorf("Expected %v Got %v", expect, got)
	}
}

func TestCleanWordsNormalized(t *testing.T) {
	var got, expect []string
	got = cleanWords("TWO   wOrds")
	expect = []string{"two", "words"}
	if !equalArrays(got, expect) {
		t.Errorf("Expected %v Got %v", expect, got)
	}
}

func TestCleanWordsCleaned(t *testing.T) {
	var got, expect []string
	got = cleanWords(" word\"[](){}?!,-:;,' ")
	expect = []string{"word"}
	if equalArrays(got, expect) != true {
		t.Errorf("Expected %v Got %v", expect, got)
	}
	got = cleanWords("sentence.")
	expect = []string{"sentence"}
	if !equalArrays(got, expect) {
		t.Errorf("Expected %v Got %v", expect, got)
	}
}

func TestCleanWordsUnidecoded(t *testing.T) {
	var got, expect []string
	got = cleanWords(" trött ")
	expect = []string{"trött", "trott"}
	if !equalArrays(got, expect) {
		t.Errorf("Expected %v Got %v", expect, got)
	}
}

func TestEncodeString(t *testing.T) {
	got := encodeString("www.peterbe.com")
	expect := "eb9f7473" // first 8 characters
	if got != expect {
		t.Errorf("Expected %v Got %v", expect, got)
	}
}

func TestGetPrefixes(t *testing.T) {
	got := getPrefixes("word")
	expect := []string{"w", "wo", "wor", "word"}
	if !equalArrays(got, expect) {
		t.Errorf("Expected %v Got %v", expect, got)
	}
}
