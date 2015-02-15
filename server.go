package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/fiam/gounidecode/unidecode"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/gorilla/mux"
	"github.com/mholt/binding"
	"github.com/namsral/flag"
	"github.com/unrolled/render"
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
	return hex.EncodeToString(h.Sum(nil))[0:8]
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

func indexHandler(w http.ResponseWriter, req *http.Request) {
	// this assumes there's a `templates/index.tmpl` file
	renderer.HTML(w, http.StatusOK, "index", map[string]string{"staticPrefix": staticPrefix})
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
	group := form.Group

	// if len(form.Group) != 0 {
	// 	groups = append(groups, strings.Split(form.Groups, ",")...)
	// }
	// sort.Strings(groups)

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.Put(c)

	domain, err := authKeys.GetDomain(key, c)
	if err != nil {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	encoded := encodeString(domain)
	encodedURL := encodeString(form.URL)

	title, _ := c.Cmd("HGET", encoded+"$titles", encodedURL).Str()
	if title == "" {
		err = c.Cmd("HINCRBY", "$domaindocuments", domain, 1).Err
		errHndlr(err)
	}

	pipedCommands := 0
	for _, prefix := range getPrefixes(form.Title) {
		if group != "" {

			encodedGroup := encodeString(group)
			c.Append("ZADD", encoded+encodedGroup+prefix, form.Popularity, encodedURL)
		} else {
			c.Append("ZADD", encoded+prefix, form.Popularity, encodedURL)
		}
		pipedCommands++
	}
	c.Append("HSET", encoded+"$titles", encodedURL, form.Title)
	pipedCommands++
	c.Append("HSET", encoded+"$urls", encodedURL, form.URL)
	pipedCommands++
	for i := 1; i <= pipedCommands; i++ {
		if err := c.GetReply().Err; err != nil {
			errHndlr(err)
		}
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
	// form.Domain = strings.Trim(form.Domain, " ")
	form.URL = strings.Trim(form.URL, " ")

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.CarefullyPut(c, &err)

	domain, err := authKeys.GetDomain(key, c)
	if err != nil {
		output := map[string]string{"error": "Auth-Key not recognized"}
		renderer.JSON(w, http.StatusForbidden, output)
		return
	}

	encoded := encodeString(domain)
	encodedURL := encodeString(form.URL)
	var title string
	title, err = c.Cmd("HGET", encoded+"$titles", encodedURL).Str()
	errHndlr(err)
	if title != "" {
		err = c.Cmd("HINCRBY", "$domaindocuments", domain, -1).Err
		errHndlr(err)
	}

	prefixes := getPrefixes(title)
	pipedCommands := 0
	for _, prefix := range prefixes {
		c.Append("ZREM", encoded+prefix, encodedURL)
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

	domain, err := authKeys.GetDomain(key, c)
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

var (
	redisPool    *pool.Pool
	procs        int
	debug        = true
	renderer     = render.New()
	redisURL     = "127.0.0.1:6379"
	authKeys     *AuthKeys
	staticPrefix = ""
)

func main() {
	var (
		port          = 3001
		redisDatabase = 0
		redisPoolSize = 10
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
	flag.Parse()

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

	authKeys = new(AuthKeys)
	authKeys.Init()

	mux := mux.NewRouter()
	mux.HandleFunc("/", indexHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", fetchHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", updateHandler).Methods("POST", "PUT")
	mux.HandleFunc("/v1", deleteHandler).Methods("DELETE")
	mux.HandleFunc("/v1/stats", privateStatsHandler).Methods("GET")

	n := negroni.Classic()

	n.UseHandler(mux)
	n.Run(fmt.Sprintf(":%d", port))
}
