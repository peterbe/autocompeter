package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/fiam/gounidecode/unidecode"
	"github.com/fzzy/radix/redis"
	"github.com/mholt/binding"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Consider refactor this from
// https://code.google.com/p/spell-correct-in-go/source/browse/trunk/spell-correct.go
func editOnes(word string) []string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	set := make(map[string]struct{})
	// n := len(word)

	join := func(slice []string) string {
		return strings.Join(slice, "")
	}

	splitted := strings.Split(word, "")
	splittedLength := len(splitted)
	var variant string
	alphabetSplitted := strings.Split(alphabet, "")

	// deletion
	for i := range splitted {
		// fmt.Println(i, e)
		// variant = strings.Join(splitted[0:i], "")
		variant = join(splitted[0:i])
		variant += join(splitted[i+1 : splittedLength])
		set[variant] = struct{}{}
	}

	// transposition
	for i := range splitted {
		if i < splittedLength-1 {
			variant = join(splitted[0:i])
			variant += join(splitted[i+1 : i+2])
			variant += join(splitted[i : i+1])
			variant += join(splitted[i+2 : splittedLength])

			// variant += strings.Join()
			// fmt.Println(i, splittedLength)
			// fmt.Println(variant)
			set[variant] = struct{}{}
		}
	}

	// replaces
	for i := range splitted {
		if i < splittedLength-1 {
			for j := range alphabetSplitted {
				variant = join(splitted[0:i])
				variant += join(alphabetSplitted[j : j+1])
				variant += join(splitted[i+1 : splittedLength])
				set[variant] = struct{}{}
			}
		}

	}

	// insertion
	for i := range splitted {
		for j := range alphabetSplitted {
			variant = join(splitted[0:i])
			variant += join(alphabetSplitted[j : j+1])
			variant += join(splitted[i:splittedLength])
			set[variant] = struct{}{}
		}
	}

	// return as a list
	list := make([]string, len(set))
	i := 0
	for k := range set {
		list[i] = k
		i++
	}
	return list
}

func encodeString(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))[0:6]
}

// With this we can remove all apostraphes that are used like
// quotation marks. E.g. "The 'one' word" or "'One' is a word".
// However, we'll want to keep apostraphes when they're in a word
// like "they're" or "there's" or "o'clock" as if it's 1 word.
var quotationMarksRegex = regexp.MustCompile(`\B'|'\B`)

var junkRegex = regexp.MustCompile(`[\[\](){}"\.?!,:;,-]`)

func cleanWords(query string) ([]string, bool) {
	query = quotationMarksRegex.ReplaceAllString(query, "")
	query = junkRegex.ReplaceAllString(query, " ")
	split := strings.Fields(strings.Trim(query, " "))
	terms := make([]string, len(split))

	var asciiTerm string
	unidecodeExpanded := false
	for i, term := range split {
		terms[i] = strings.ToLower(strings.Trim(strings.Trim(term, " "), "."))
		asciiTerm = unidecode.Unidecode(terms[i])
		if asciiTerm != terms[i] {
			terms = append(terms, asciiTerm)
			unidecodeExpanded = true
		}
	}
	return terms, unidecodeExpanded
}

// func getPrefixes(title string) []string {
// 	var prefixes []string
// 	words, _ := cleanWords(title)
// 	for _, word := range words {
// 		for i := range word {
// 			if i > 0 {
// 				prefixes = append(prefixes, word[0:i])
// 			}
// 		}
// 		prefixes = append(prefixes, word) // should we keep this?!
// 		prefixes = append(prefixes, word+"$")
// 	}
// 	return prefixes
// }

type prefix struct {
	start string
	term  string
}

func getPrefixes(title string) []prefix {
	//var prefixes []string
	var prefixes []prefix
	words, _ := cleanWords(title)
	for _, word := range words {
		for i := range word {
			if i > 0 {
				//prefixes = append(prefixes, word[0:i])
				prefixes = append(prefixes, prefix{word[0:i], word})
			}
		}
		prefixes = append(prefixes, prefix{word, word}) // should we keep this?!
		prefixes = append(prefixes, prefix{word + "$", word})
	}
	return prefixes
}

func pingHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(w, "pong\n")
}

type updateForm struct {
	URL        string
	Title      string
	Group      string
	Popularity float64
}

func (f *updateForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.URL: binding.Field{
			Form:     "url",
			Required: true,
		},
		&f.Title: binding.Field{
			Form:     "title",
			Required: true,
		},
		&f.Group:      "group",
		&f.Popularity: "popularity",
	}
}

func (f updateForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if strings.Trim(f.Title, " ") == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"title"},
			Classification: "ComplaintError",
			Message:        "Can't be empty",
		})
	}
	if strings.Trim(f.URL, " ") == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"url"},
			Classification: "ComplaintError",
			Message:        "Can't be empty",
		})
	}
	return errs
}

func updateHandler(w http.ResponseWriter, req *http.Request) {
	key := req.Header.Get("AUTH-KEY")
	if key == "" {
		output := map[string]string{"error": "Auth-Key header not set"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}
	form := new(updateForm)
	errs := binding.Bind(req, form)
	if errs.Handle(w) {
		return
	}
	form.Title = strings.Trim(form.Title, " ")
	form.URL = strings.Trim(form.URL, " ")
	// group := form.Group

	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.Put(c)

	domain, err := GetDomain(key, c)
	if domain == "" {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	insertDocument(
		domain,
		form.Title,
		form.URL,
		form.Group,
		form.Popularity,
		c,
	)

	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusCreated, output)
}

func insertDocument(domain, title, url, group string, popularity float64, c *redis.Client) {
	encoded := encodeString(domain)
	encodedURL := encodeString(url)

	existingTitle, _ := c.Cmd("HGET", encoded+"$titles", encodedURL).Str()
	if existingTitle == "" {
		err := c.Cmd("HINCRBY", "$domaindocuments", domain, 1).Err
		errorHandler(err)
	}

	pipedCommands := 0
	// var encodedURLAndTerm string
	for _, prefix := range getPrefixes(title) {
		// encodedURLAndTerm = fmt.Sprintf("%v**%v", encodedURL, prefix.term)
		if group != "" {
			encodedGroup := encodeString(group)
			c.Append("ZADD", encoded+encodedGroup+prefix.start, popularity, encodedURL)
			c.Append("ZADD", encoded+encodedGroup+"$term"+prefix.start, popularity, prefix.term)
			c.Append("HSET", encoded+"$groups", encodedURL, encodedGroup)
			pipedCommands++
		} else {
			c.Append("ZADD", encoded+prefix.start, popularity, encodedURL)
			c.Append("ZADD", encoded+"$term"+prefix.start, popularity, prefix.term)
		}
		pipedCommands++
		pipedCommands++
	}
	c.Append("HSET", encoded+"$titles", encodedURL, title)
	pipedCommands++
	c.Append("HSET", encoded+"$urls", encodedURL, url)
	pipedCommands++
	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			fmt.Println("ERROR!!!")
			errorHandler(err)
		}
	}

}

type bulkDocuments struct {
	Documents []bulkDocument `json:"documents"`
}

type bulkDocument struct {
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Popularity float64 `json:"popularity"`
	Group      string  `json:"group"`
}

func bulkHandler(w http.ResponseWriter, req *http.Request) {
	key := req.Header.Get("AUTH-KEY")
	if key == "" {
		output := map[string]string{"error": "Auth-Key header not set"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}
	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.Put(c)

	domain, err := GetDomain(key, c)
	if domain == "" {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	// encoded := encodeString(domain)

	decoder := json.NewDecoder(req.Body)
	var bs bulkDocuments
	err = decoder.Decode(&bs)
	errorHandler(err)
	for _, b := range bs.Documents {
		insertDocument(
			domain,
			b.Title,
			b.URL,
			b.Group,
			b.Popularity,
			c,
		)
	}
	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusCreated, output)
}

type deleteForm struct {
	// Domain string
	URL string
}

// FieldMap defines the bindings for deleteForm
func (f *deleteForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		// &f.Domain: binding.Field{
		// 	Form:     "domain",
		// 	Required: true,
		// },
		&f.URL: binding.Field{
			Form:     "url",
			Required: true,
		},
	}
}

// Validate checks the values for deleteForm
func (f deleteForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// if strings.Trim(f.Domain, " ") == "" {
	// 	errs = append(errs, binding.Error{
	// 		FieldNames:     []string{"domain"},
	// 		Classification: "ComplaintError",
	// 		Message:        "Can't be empty",
	// 	})
	// }
	if strings.Trim(f.URL, " ") == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"url"},
			Classification: "ComplaintError",
			Message:        "Can't be empty",
		})
	}
	return errs
}

func deleteHandler(w http.ResponseWriter, req *http.Request) {
	key := req.Header.Get("AUTH-KEY")
	if key == "" {
		output := map[string]string{"error": "Auth-Key header not set"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}
	form := new(deleteForm)
	errs := binding.Bind(req, form)
	if errs.Handle(w) {
		return
	}
	form.URL = strings.Trim(form.URL, " ")

	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.CarefullyPut(c, &err)

	domain, err := GetDomain(key, c)
	if domain == "" {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	encoded := encodeString(domain)
	encodedURL := encodeString(form.URL)
	var title string
	reply := c.Cmd("HGET", encoded+"$titles", encodedURL)
	if reply.Type == redis.NilReply {
		output := map[string]string{"error": "URL not recognized"}
		renderer.JSON(w, http.StatusNotFound, output)
		return
	}
	title, err = reply.Str()

	errorHandler(err)
	if title != "" {
		err = c.Cmd("HINCRBY", "$domaindocuments", domain, -1).Err
		errorHandler(err)
	}

	reply = c.Cmd("HGET", encoded+"$groups", encodedURL)
	encodedGroup := ""
	if reply.Type != redis.NilReply {
		encodedGroup, _ = reply.Str()
	}
	prefixes := getPrefixes(title)
	pipedCommands := 0
	// var encodedURLAndTerm string
	for _, prefix := range prefixes {
		// encodedURLAndTerm = fmt.Sprintf("%v**%v", encodedURL, prefix.term)
		if encodedGroup != "" {
			c.Append("ZREM", encoded+encodedGroup+prefix.start, encodedURL)
			c.Append("ZREM", encoded+encodedGroup+"$term"+prefix.start, prefix.term)
			c.Append("HDEL", encoded+"$groups", encodedURL)
			pipedCommands++
		} else {
			c.Append("ZREM", encoded+"$term"+prefix.start, encodedURL)
			c.Append("ZREM", encoded+prefix.start, encodedURL)
		}
		pipedCommands++
		pipedCommands++
	}

	c.Append("HDEL", encoded+"$titles", encodedURL)
	pipedCommands++
	c.Append("HDEL", encoded+"$urls", encodedURL)
	pipedCommands++

	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			errorHandler(err)
		}
	}
	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusNoContent, output)
}

// Reply isn't a great name :)
type Reply struct {
	URL   string
	Score string
	// Term  string
}

type fetchForm struct {
	Number int
	Groups string
	Query  string
	Domain string
}

// FieldMap defines the bindings for fetchForm
func (f *fetchForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.Number: "n",
		&f.Groups: "g",
		&f.Query: binding.Field{
			Form:     "q",
			Required: true,
		},
		&f.Domain: binding.Field{
			Form:     "d",
			Required: true,
		},
	}
}

func fetchHandler(w http.ResponseWriter, req *http.Request) {
	// time.Sleep(1 * time.Second)
	// time.Sleep(60  *   time.Millisecond)
	form := new(fetchForm)
	errs := binding.Bind(req, form)
	if errs.Handle(w) {
		return
	}
	n := form.Number
	if n <= 0 {
		n = 10 // default
	}

	form.Domain = strings.Trim(form.Domain, " ")
	groups := []string{}
	if len(form.Groups) != 0 {
		groups = append(groups, strings.Split(form.Groups, ",")...)
	}
	sort.Strings(groups)

	encoded := encodeString(form.Domain)

	form.Query = strings.Trim(form.Query, " ")
	terms, unidecodeExpanded := cleanWords(form.Query)
	// searchedTerms := make([]string, len(terms))
	// copy(searchedTerms, terms)
	// var searchedTerms []string

	// this is only temporary logging
	ts := time.Now().Unix()
	log.Printf("%v$%v$%v", form.Domain, form.Query, ts)

	// If the queryhas more than one term, e.g "one spo" then we have
	// completed the first word and don't expect autocompletion on that.
	// For example, it should now find "one spotless thing" but not
	// "oneanother sport".
	// And because every word has been indexed in its complete form
	// we should have a key called "one$" and a key called "spo" that
	// we can search on.
	if len(terms) > 1 && !unidecodeExpanded {
		for i, term := range terms {
			if i+1 < len(terms) {
				terms[i] = term + "$"
			}
		}
	}

	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.CarefullyPut(c, &err)

	now := time.Now()
	thisMonthFetchesKey := fmt.Sprintf("$domainfetches$%v$%v", now.Year(), int(now.Month()))
	err = c.Cmd("HINCRBY", thisMonthFetchesKey, form.Domain, 1).Err
	errorHandler(err)

	getReplies := func(terms []string, group string, useUnion bool, termSearch bool) ([]string, error) {
		encodedTerms := make([]string, len(terms))
		// encodedTermTerms := make([]string, len(terms))
		encodedGroup := ""
		if group != "" {
			encodedGroup = encodeString(group)
		}
		for i, term := range terms {
			if termSearch {
				encodedTerms[i] = encoded + encodedGroup + "$term" + term
			} else {
				encodedTerms[i] = encoded + encodedGroup + term
			}
		}
		var replies []string
		// fmt.Println("encodedTerms", encodedTerms)
		if len(terms) > 1 {
			if useUnion {
				// fmt.Println("ZUNIONSTORE", "$tmp", len(terms), encodedTerms, "AGGREGATE", "max")
				c.Append("ZUNIONSTORE", "$tmp", len(terms), encodedTerms, "AGGREGATE", "max")
			} else {
				// fmt.Println("ZINTERSTORE", "$tmp", len(terms), encodedTerms, "AGGREGATE", "max")
				c.Append("ZINTERSTORE", "$tmp", len(terms), encodedTerms, "AGGREGATE", "max")
			}
			c.Append("ZREVRANGE", "$tmp", 0, n-1, "WITHSCORES")
			c.GetReply() // the ZINTERSTORE
			replies, err = c.GetReply().List()
		} else {
			replies, err = c.Cmd("ZREVRANGE", encodedTerms[0], 0, n-1, "WITHSCORES").List()
		}
		return replies, err
	}

	var replies []string
	var termsReplies []string
	if len(terms) > 0 {
		replies, err = getReplies(terms, "", false, false)
		errorHandler(err)
		// fmt.Println("REPLIES", replies)
		termsReplies, err = getReplies(terms, "", true, true)
		errorHandler(err)
	}
	var replyStructs []Reply
	var termsFound []string
	for i, element := range replies {
		if i%2 == 0 {
			replyStructs = append(replyStructs, Reply{element, replies[i+1]})
		}
	}
	for i, element := range termsReplies {
		if i%2 == 0 {
			termsFound = append(termsFound, element)
		}
	}
	if len(replyStructs) == 0 && len(terms) == 1 && len(terms[0]) > 2 && len(terms[0]) <= 6 {
		variations := editOnes(terms[0])
		// fmt.Println(variations)
		replies, err = getReplies(variations, "", true, false)
		errorHandler(err)
		for i, element := range replies {
			if i%2 == 0 {
				// splitted = strings.SplitN(element, "**", 2)
				// URL = splitted[0]
				// termUsed = splitted[1]
				replyStructs = append(replyStructs, Reply{element, replies[i+1]})
				// searchedTerms = append(searchedTerms, termUsed)
			}
		}
		termsReplies, err = getReplies(variations, "", true, true)
		errorHandler(err)
		for i, element := range termsReplies {
			if i%2 == 0 {
				termsFound = append(termsFound, element)
			}
		}
	}

	for _, group := range groups {
		replies, err = getReplies(terms, group, false, false)
		errorHandler(err)
		for i, element := range replies {
			if i%2 == 0 {
				replyStructs = append(replyStructs, Reply{element, replies[i+1]})
			}
		}
		termsReplies, err = getReplies(terms, group, true, true)
		errorHandler(err)
		for i, element := range replies {
			if i%2 == 0 {
				termsFound = append(termsFound, element)
			}
		}
	}
	// fmt.Println("replyStructs", replyStructs)
	RemoveDuplicateReplyStructs := func(xs *[]Reply) {
		found := make(map[string]bool)
		j := 0
		for i, x := range *xs {
			if !found[x.URL] {
				found[x.URL] = true
				(*xs)[j] = (*xs)[i]
				j++
			}
		}
		*xs = (*xs)[:j]
	}
	RemoveDuplicateReplyStructs(&replyStructs)
	if len(replyStructs) > n {
		replyStructs = replyStructs[:n]
	}

	// We might want to sort this here by the extra business logic
	// on sorting.
	encodedUrls := make([]string, len(replyStructs))
	for i, each := range replyStructs {
		encodedUrls[i] = each.URL
	}

	var titles []string
	var urls []string
	if len(encodedUrls) == 0 {
	} else {
		titles, err = c.Cmd("HMGET", encoded+"$titles", encodedUrls).List()
		errorHandler(err)
		urls, err = c.Cmd("HMGET", encoded+"$urls", encodedUrls).List()
		errorHandler(err)
	}
	rows := make([]interface{}, len(titles))
	for i, title := range titles {
		row := make([]string, 2)
		row[0] = urls[i]
		row[1] = title
		rows[i] = row
	}
	rows = rows[:len(titles)]

	// RemoveDuplicatesStrings := func(xs *[]string) {
	// 	found := make(map[string]bool)
	// 	j := 0
	// 	for i, x := range *xs {
	// 		if !found[x] {
	// 			found[x] = true
	// 			(*xs)[j] = (*xs)[i]
	// 			j++
	// 		}
	// 	}
	// 	*xs = (*xs)[:j]
	// }

	output := make(map[string]interface{})
	// fmt.Println("termsFound", termsFound)
	if len(termsFound) == 0 {
		// otherwise the json package turns this into null
		output["terms"] = []string{}
	} else {
		// RemoveDuplicatesStrings(&termsFound)
		output["terms"] = termsFound
	}
	output["results"] = rows
	w.Header().Set("Access-Control-Allow-Origin", "*")
	renderer.JSON(w, http.StatusOK, output)
}

func privateStatsHandler(w http.ResponseWriter, req *http.Request) {
	key := req.Header.Get("AUTH-KEY")
	if key == "" {
		output := map[string]string{"error": "Auth-Key header not set"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.Put(c)

	domain, err := GetDomain(key, c)
	if err != nil {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	documents := 0
	documentsStr, err := c.Cmd("HGET", "$domaindocuments", domain).Str()
	if documentsStr != "" {
		documents, err = strconv.Atoi(documentsStr)
		errorHandler(err)
	}

	now := time.Now()
	var dt time.Time
	allFetches := make(map[string]interface{})
	var fetchKey string
	var fetchesStr string
	var fetches int
	// starting on the year 2015 because that's when it all started
	for y := 2015; y <= now.Year(); y++ {
		thisYearFetches := make(map[string]int)
		for m := 1; m <= 12; m++ {
			dt = time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
			if dt.Before(now) {
				fetchKey = fmt.Sprintf("$domainfetches$%v$%v", dt.Year(), int(dt.Month()))
				fetchesStr, err = c.Cmd("HGET", fetchKey, domain).Str()
				if err == nil {
					fetches, err = strconv.Atoi(fetchesStr)
					errorHandler(err)
					thisYearFetches[fmt.Sprintf("%v", m)] = fetches
				}
			}
		}
		allFetches[fmt.Sprintf("%v", y)] = thisYearFetches
	}
	output := make(map[string]interface{})
	output["fetches"] = allFetches
	output["documents"] = documents
	w.Header().Set("Access-Control-Allow-Origin", "*")
	renderer.JSON(w, http.StatusOK, output)

}

func flushHandler(w http.ResponseWriter, req *http.Request) {
	key := req.Header.Get("AUTH-KEY")
	if key == "" {
		output := map[string]string{"error": "Auth-Key header not set"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.Put(c)

	domain, err := GetDomain(key, c)
	if err != nil {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	encoded := encodeString(domain)

	all, err := c.Cmd("HGETALL", encoded+"$titles").List()
	errorHandler(err)
	pipedCommands := 0
	var encodedURL string
	// var encodedURLAndTerm string
	for i, each := range all {
		if i%2 == 0 {
			encodedURL = each
		} else {
			encodedGroup := ""
			reply := c.Cmd("HGET", encoded+"$groups", encodedURL)
			if reply.Type != redis.NilReply {
				encodedGroup, err = reply.Str()
				errorHandler(err)
			}
			prefixes := getPrefixes(each)
			for _, prefix := range prefixes {
				// encodedURLAndTerm = fmt.Sprintf("%v**%v", encodedURL, prefix.term)
				if encodedGroup != "" {
					c.Append("ZREM", encoded+encodedGroup+prefix.start, encodedURL)
					c.Append("ZREM", encoded+encodedGroup+prefix.start, encodedURL)
					c.Append("ZREM", encoded+encodedGroup+"$term"+prefix.start, prefix.term)
					pipedCommands++
				} else {
					c.Append("ZREM", encoded+prefix.start, encodedURL)
					c.Append("ZREM", encoded+"$term"+prefix.start, prefix.term)
				}
				pipedCommands++
				pipedCommands++
			}
			c.Append("HDEL", encoded+"$titles", encodedURL)
			pipedCommands++
			c.Append("HDEL", encoded+"$urls", encodedURL)
			pipedCommands++
			if encodedGroup != "" {
				c.Append("HDEL", encoded+"$groups", encodedURL)
				pipedCommands++
			}
		}
	}

	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			errorHandler(err)
		}
	}

	err = c.Cmd("HSET", "$domaindocuments", domain, 0).Err
	errorHandler(err)

	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusNoContent, output)
}
