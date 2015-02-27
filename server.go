package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/fiam/gounidecode/unidecode"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/mholt/binding"
	"github.com/namsral/flag"
	"github.com/unrolled/render"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
	"math/rand"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var junkRegex = regexp.MustCompile(`[\[\](){}"?!,-:;,']`)

func cleanWords(query string) ([]string, bool) {
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

func encodeString(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))[0:6]
}

func getPrefixes(title string) []string {
	var prefixes []string
	words, _ := cleanWords(title)
	for _, word := range words {
		for i := 1; i <= len(word); i++ {
			prefixes = append(prefixes, word[0:i])
		}
		prefixes = append(prefixes, word+"$")
	}
	return prefixes
}

// func QueryScore(terms []string, title) float32 {
// 	return 1.0
// }

func errHndlr(err error) {
	if err != nil {
		fmt.Println("error:", err)
		panic(err)
	}
}

func isOnHTTPS(req *http.Request) bool {
	if req.Header.Get("is-secure") == "true" {
		return true
	}
	// default is to use the flag
	// which is only really useful for local development
	return usingHTTPS
}

type domainRow struct {
	Key    string
	Domain string
}

func indexHandler(w http.ResponseWriter, req *http.Request) {
	context := map[string]interface{}{
		"staticPrefix": staticPrefix,
		"isNotDebug":   !debug,
		"Username":     "",
		"domains":      make([]string, 0),
	}

	cookie, err := req.Cookie("username")
	if err == nil {
		context["Username"] = cookie.Value
		c, err := redisPool.Get()
		errHndlr(err)
		defer redisPool.CarefullyPut(c, &err)

		userdomainsKey := fmt.Sprintf("$userdomains$%v", cookie.Value)
		replies, err := c.Cmd("SMEMBERS", userdomainsKey).List()
		errHndlr(err)

		domains := make([]domainRow, len(replies))

		var domain string
		for i, key := range replies {
			reply := c.Cmd("HGET", "$domainkeys", key)
			if reply.Type != redis.NilReply {
				domain, err = reply.Str()
				errHndlr(err)
				domains[i] = domainRow{
					Key:    key,
					Domain: domain,
				}
			}
		}
		context["domains"] = domains
	}
	// this assumes there's a `templates/index.tmpl` file
	renderer.HTML(w, http.StatusOK, "index", context)
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
	errHndlr(err)
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
		errHndlr(err)
	}

	pipedCommands := 0
	for _, prefix := range getPrefixes(title) {
		if group != "" {
			encodedGroup := encodeString(group)
			c.Append("ZADD", encoded+encodedGroup+prefix, popularity, encodedURL)
			c.Append("HSET", encoded+"$groups", encodedURL, encodedGroup)
			pipedCommands++
		} else {
			c.Append("ZADD", encoded+prefix, popularity, encodedURL)
		}
		pipedCommands++
	}
	c.Append("HSET", encoded+"$titles", encodedURL, title)
	pipedCommands++
	c.Append("HSET", encoded+"$urls", encodedURL, url)
	pipedCommands++
	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			errHndlr(err)
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
	errHndlr(err)
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
	errHndlr(err)
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
	errHndlr(err)
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

	errHndlr(err)
	if title != "" {
		err = c.Cmd("HINCRBY", "$domaindocuments", domain, -1).Err
		errHndlr(err)
	}

	reply = c.Cmd("HGET", encoded+"$groups", encodedURL)
	encodedGroup := ""
	if reply.Type != redis.NilReply {
		encodedGroup, _ = reply.Str()
	}
	prefixes := getPrefixes(title)
	pipedCommands := 0
	for _, prefix := range prefixes {
		if encodedGroup != "" {
			c.Append("ZREM", encoded+encodedGroup+prefix, encodedURL)
			c.Append("HDEL", encoded+"$groups", encodedURL)
			pipedCommands++
		} else {
			c.Append("ZREM", encoded+prefix, encodedURL)
		}

		pipedCommands++
	}

	c.Append("HDEL", encoded+"$titles", encodedURL)
	pipedCommands++
	c.Append("HDEL", encoded+"$urls", encodedURL)
	pipedCommands++

	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			errHndlr(err)
		}
	}
	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusNoContent, output)
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

// Reply isn't a great name :)
type Reply struct {
	URL   string
	Score string
}

func fetchHandler(w http.ResponseWriter, req *http.Request) {
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
	searchedTerms := make([]string, len(terms))
	copy(searchedTerms, terms)

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
	errHndlr(err)
	defer redisPool.CarefullyPut(c, &err)

	now := time.Now()
	thisMonthFetchesKey := fmt.Sprintf("$domainfetches$%v$%v", now.Year(), int(now.Month()))
	err = c.Cmd("HINCRBY", thisMonthFetchesKey, form.Domain, 1).Err
	errHndlr(err)

	getReplies := func(terms []string, group string) ([]string, error) {
		encodedTerms := make([]string, len(terms))
		encodedGroup := ""
		if group != "" {
			encodedGroup = encodeString(group)
		}
		for i, term := range terms {
			encodedTerms[i] = encoded + encodedGroup + term
		}
		var replies []string
		if len(terms) > 1 {
			c.Append("ZINTERSTORE", "$tmp", len(terms), encodedTerms, "AGGREGATE", "max")
			c.Append("ZREVRANGE", "$tmp", 0, n-1, "WITHSCORES")
			c.GetReply() // the ZINTERSTORE
			replies, err = c.GetReply().List()
		} else {
			replies, err = c.Cmd("ZREVRANGE", encodedTerms[0], 0, n-1, "WITHSCORES").List()
		}
		return replies, err
	}

	replies, err := getReplies(terms, "")
	errHndlr(err)
	var replyStructs []Reply
	for i, element := range replies {
		if i%2 == 0 {
			replyStructs = append(replyStructs, Reply{element, replies[i+1]})
		}
	}
	for _, group := range groups {
		replies, err = getReplies(terms, group)
		errHndlr(err)
		for i, element := range replies {
			if i%2 == 0 {
				replyStructs = append(replyStructs, Reply{element, replies[i+1]})
			}
		}
	}
	RemoveDuplicates := func(xs *[]Reply) {
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
	RemoveDuplicates(&replyStructs)
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
		errHndlr(err)
		urls, err = c.Cmd("HMGET", encoded+"$urls", encodedUrls).List()
		errHndlr(err)
	}
	rows := make([]interface{}, len(titles))
	for i, title := range titles {
		row := make([]string, 2)
		row[0] = urls[i]
		row[1] = title
		rows[i] = row
	}
	rows = rows[:len(titles)]

	output := make(map[string]interface{})
	output["terms"] = searchedTerms
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
	errHndlr(err)
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
		errHndlr(err)
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
					errHndlr(err)
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
	errHndlr(err)
	defer redisPool.Put(c)

	domain, err := GetDomain(key, c)
	if err != nil {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	encoded := encodeString(domain)

	all, err := c.Cmd("HGETALL", encoded+"$titles").List()
	errHndlr(err)
	pipedCommands := 0
	var encodedURL string
	for i, each := range all {
		if i%2 == 0 {
			encodedURL = each
		} else {
			encodedGroup := ""
			reply := c.Cmd("HGET", encoded+"$groups", encodedURL)
			if reply.Type != redis.NilReply {
				encodedGroup, err = reply.Str()
				errHndlr(err)
			}
			prefixes := getPrefixes(each)
			for _, prefix := range prefixes {
				if encodedGroup != "" {
					c.Append("ZREM", encoded+encodedGroup+prefix, encodedURL)
				} else {
					c.Append("ZREM", encoded+prefix, encodedURL)
				}
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

	now := time.Now()
	// var dt time.Time
	var fetchKey string
	for y := 2015; y <= now.Year(); y++ {
		for m := 1; m <= 12; m++ {
			fetchKey = fmt.Sprintf("$domainfetches$%v$%v", y, m)
			c.Append("HDEL", fetchKey, domain)
			pipedCommands++
		}
	}

	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			errHndlr(err)
		}
	}

	err = c.Cmd("HSET", "$domaindocuments", domain, 0).Err
	errHndlr(err)

	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusNoContent, output)
}

func logoutHandler(w http.ResponseWriter, req *http.Request) {
	expire := time.Now().AddDate(0, 0, -1)
	secureCookie := isOnHTTPS(req)
	cookie := &http.Cookie{
		Name:     "username",
		Value:    "*deleted*",
		Path:     "/",
		Expires:  expire,
		MaxAge:   -1,
		Secure:   secureCookie,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, req, "/#loggedout", http.StatusTemporaryRedirect)
}

func handleGitHubLogin(w http.ResponseWriter, req *http.Request) {
	url := oauthConf.AuthCodeURL(oauthStateString, oauth2.AccessTypeOnline)
	http.Redirect(w, req, url, http.StatusTemporaryRedirect)
}

func handleGitHubCallback(w http.ResponseWriter, req *http.Request) {
	state := req.FormValue("state")
	if state != oauthStateString {
		fmt.Printf("invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	code := req.FormValue("code")
	token, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		fmt.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	oauthClient := oauthConf.Client(oauth2.NoContext, token)
	client := github.NewClient(oauthClient)
	// the second item here is the github.Rate config
	user, _, err := client.Users.Get("")

	if err != nil {
		fmt.Printf("client.Users.Get() faled with '%s'\n", err)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	fmt.Printf("Logged in as GitHub user: %s\n", *user.Login)
	// userstate.Login(w, *user.Login)
	expire := time.Now().AddDate(0, 0, 1) // how long is this?
	secureCookie := isOnHTTPS(req)
	cookie := &http.Cookie{
		Name:     "username",
		Value:    *user.Login,
		Path:     "/",
		Expires:  expire,
		MaxAge:   60 * 60 * 24 * 30, // 30 days
		Secure:   secureCookie,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, req, "/#auth", http.StatusTemporaryRedirect)
}

var letters = []rune(
	"abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ123456789",
)

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func domainkeyNewHandler(w http.ResponseWriter, req *http.Request) {
	domain := strings.Trim(req.FormValue("domain"), " ")
	if domain != "" {
		cookie, err := req.Cookie("username")
		if err == nil {
			c, err := redisPool.Get()
			errHndlr(err)
			defer redisPool.CarefullyPut(c, &err)

			key := randString(24)
			userdomainsKey := fmt.Sprintf("$userdomains$%v", cookie.Value)
			err = c.Cmd("SADD", userdomainsKey, key).Err
			errHndlr(err)
			err = c.Cmd("HSET", "$domainkeys", key, domain).Err
			errHndlr(err)
		}
	}

	// http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
	http.Redirect(w, req, "/#auth", http.StatusFound)
}

func domainkeyDeleteHandler(w http.ResponseWriter, req *http.Request) {
	key := strings.Trim(req.FormValue("key"), " ")
	if key != "" {
		cookie, err := req.Cookie("username")
		if err == nil {
			c, err := redisPool.Get()
			errHndlr(err)
			defer redisPool.CarefullyPut(c, &err)

			userdomainsKey := fmt.Sprintf("$userdomains$%v", cookie.Value)
			err = c.Cmd("SREM", userdomainsKey, key).Err
			errHndlr(err)
			err = c.Cmd("HDEL", "$domainkeys", key).Err
			errHndlr(err)
		}
	}

	http.Redirect(w, req, "/#auth", http.StatusFound)
}

var (
	redisPool    *pool.Pool
	procs        int
	debug        = true
	renderer     = render.New()
	redisURL     = "127.0.0.1:6379"
	staticPrefix = ""
	usingHTTPS   = false
)

var (
	// You must register the app at https://github.com/settings/applications
	// Set callback to http://127.0.0.1:7000/github_oauth_cb
	// Set ClientId and ClientSecret to
	oauthConf = &oauth2.Config{
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{"user:email"},
		Endpoint:     githuboauth.Endpoint,
	}
	// random string for oauth2 API calls to protect against CSRF
	oauthStateString = "thisshouldberandom"
)

func main() {
	var (
		port          = 3001
		redisDatabase = 0
		redisPoolSize = 10
		clientID      = ""
		clientSecret  = ""
	)
	flag.IntVar(&port, "port", port, "Port to start the server on")
	flag.IntVar(&procs, "procs", 1, "Number of CPU processors (0 to use max)")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.StringVar(
		&redisURL, "redisURL", redisURL,
		"Redis URL to tcp connect to")
	flag.StringVar(
		&staticPrefix, "staticPrefix", staticPrefix,
		"Prefix in front of static assets in HTML")
	flag.IntVar(&redisDatabase, "redisDatabase", redisDatabase,
		"Redis database number to connect to")
	flag.StringVar(
		&clientID, "clientID", clientID,
		"OAuth Client ID")
	flag.StringVar(
		&clientSecret, "clientSecret", clientSecret,
		"OAuth Client Secret")
	flag.BoolVar(&usingHTTPS, "usingHTTPS", usingHTTPS,
		"Whether requests are made under HTTPS")
	flag.Parse()

	oauthConf.ClientID = clientID
	oauthConf.ClientSecret = clientSecret

	fmt.Println("REDIS DATABASE:", redisDatabase)
	fmt.Println("DEBUG MODE:", debug)
	fmt.Println("STATIC PREFIX:", staticPrefix)

	if !debug {
		redisPoolSize = 100
	}

	// Figuring out how many processors to use.
	maxProcs := runtime.NumCPU()
	if procs == 0 {
		procs = maxProcs
	} else if procs < 0 {
		panic("PROCS < 0")
	} else if procs > maxProcs {
		panic(fmt.Sprintf("PROCS > max (%v)", maxProcs))
	}
	fmt.Println("PROCS:", procs)
	runtime.GOMAXPROCS(procs)

	renderer = render.New(render.Options{
		IndentJSON:    debug,
		IsDevelopment: debug,
	})

	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		err = client.Cmd("SELECT", redisDatabase).Err
		if err != nil {
			return nil, err
		}
		// if err = client.Cmd("AUTH", "SUPERSECRET").Err; err != nil {
		// 	client.Close()
		// 	return nil, err
		// }
		return client, nil
	}

	var err error
	redisPool, err = pool.NewCustomPool("tcp", redisURL, redisPoolSize, df)
	errHndlr(err)

	mux := mux.NewRouter()
	mux.HandleFunc("/", indexHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", fetchHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", updateHandler).Methods("POST", "PUT")
	mux.HandleFunc("/v1", deleteHandler).Methods("DELETE")
	mux.HandleFunc("/v1/stats", privateStatsHandler).Methods("GET")
	mux.HandleFunc("/v1/flush", flushHandler).Methods("DELETE")
	mux.HandleFunc("/v1/bulk", bulkHandler).Methods("POST", "PUT")
	mux.HandleFunc("/login", handleGitHubLogin).Methods("GET")
	mux.HandleFunc("/logout", logoutHandler).Methods("GET", "POST")
	mux.HandleFunc("/github_oauth_cb", handleGitHubCallback).Methods("GET")
	mux.HandleFunc("/domainkeys/new", domainkeyNewHandler).Methods("POST")
	mux.HandleFunc("/domainkeys/delete", domainkeyDeleteHandler).Methods("POST")

	// handleGitHubLogin
	n := negroni.Classic()

	n.UseHandler(mux)
	n.Run(fmt.Sprintf(":%d", port))
}
