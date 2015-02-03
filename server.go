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
	"strings"
)

var junkRegex = regexp.MustCompile(`[\[\](){}"?!,-:;,']`)

func cleanWords(query string) []string {
	query = junkRegex.ReplaceAllString(query, " ")
	split := strings.Fields(strings.Trim(query, " "))
	terms := make([]string, len(split))
	var asciiTerm string
	for i, term := range split {
		terms[i] = strings.ToLower(strings.Trim(strings.Trim(term, " "), "."))
		asciiTerm = unidecode.Unidecode(terms[i])
		if asciiTerm != terms[i] {
			terms = append(terms, asciiTerm)
		}
	}
	return terms
}

func encodeString(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))[0:8]
}

func getPrefixes(title string) []string {
	var prefixes []string
	for _, word := range cleanWords(title) {
		for i := 1; i <= len(word); i++ {
			prefixes = append(prefixes, word[0:i])
		}
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
	renderer.HTML(w, http.StatusOK, "index", nil)
}

type updateForm struct {
	Domain     string
	URL        string
	Title      string
	Groups     string
	Popularity float64
}

func (f *updateForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.Domain: binding.Field{
			Form:     "domain",
			Required: true,
		},
		&f.URL: binding.Field{
			Form:     "url",
			Required: true,
		},
		&f.Title: binding.Field{
			Form:     "title",
			Required: true,
		},
		&f.Groups:     "groups",
		&f.Popularity: "popularity",
	}
}

func (f updateForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if strings.Trim(f.Domain, " ") == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"domain"},
			Classification: "ComplaintError",
			Message:        "Can't be empty",
		})
	}
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
	form := new(updateForm)
	errs := binding.Bind(req, form)
	if errs.Handle(w) {
		return
	}
	form.Domain = strings.Trim(form.Domain, " ")
	form.Title = strings.Trim(form.Title, " ")
	form.URL = strings.Trim(form.URL, " ")

	encoded := encodeString(form.Domain)
	encodedURL := encodeString(form.URL)

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.Put(c)
	pipedCommands := 0
	for _, prefix := range getPrefixes(form.Title) {
		c.Append("ZADD", encoded+prefix, form.Popularity, encodedURL)
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
	Domain string
	URL    string
}

// FieldMap defines the bindings for deleteForm
func (f *deleteForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.Domain: binding.Field{
			Form:     "domain",
			Required: true,
		},
		&f.URL: binding.Field{
			Form:     "url",
			Required: true,
		},
	}
}

// Validate checks the values for deleteForm
func (f deleteForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if strings.Trim(f.Domain, " ") == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"domain"},
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

func deleteHandler(w http.ResponseWriter, req *http.Request) {
	form := new(deleteForm)
	errs := binding.Bind(req, form)
	if errs.Handle(w) {
		return
	}
	form.Domain = strings.Trim(form.Domain, " ")
	form.URL = strings.Trim(form.URL, " ")

	encoded := encodeString(form.Domain)

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.CarefullyPut(c, &err)

	encodedURL := encodeString(form.URL)
	var title string
	title, err = c.Cmd("HGET", encoded+"$titles", encodedURL).Str()
	if err != nil {
		errHndlr(err)
	}
	// fmt.Println("Retrieved Title:", title)
	prefixes := getPrefixes(title)
	// fmt.Println("Prefixes Title:", prefixes)
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
	Query  string
	Domain string
}

// FieldMap defines the bindings for fetchForm
func (f *fetchForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.Number: "n",
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

	encoded := encodeString(form.Domain)
	// fmt.Println(domain, encoded)

	form.Query = strings.Trim(form.Query, " ")
	terms := cleanWords(form.Query)

	c, err := redisPool.Get()
	errHndlr(err)
	defer redisPool.CarefullyPut(c, &err)

	encodedTerms := make([]string, len(terms))
	for i, term := range terms {
		encodedTerms[i] = encoded + term
	}
	// NOTE! Maybe we don't need the ZINTERSTORE if there's only 1 command
	c.Append("ZINTERSTORE", "$tmp", len(terms), encodedTerms, "AGGREGATE", "max")
	c.Append("ZREVRANGE", "$tmp", 0, n-1, "WITHSCORES")

	c.GetReply() // the ZINTERSTORE
	replies, err := c.GetReply().List()
	// fmt.Println("replies", replies, len(replies))
	errHndlr(err)

	encodedUrls := make([]string, n+1)
	scores := make([]string, n+1)
	evens := 0
	for i, element := range replies {
		if i%2 == 0 {
			encodedUrls[evens] = element
			evens = evens + 1
		} else {
			scores[evens-1] = element
		}
	}
	encodedUrls = encodedUrls[:evens]
	scores = scores[:evens]

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
		// fmt.Println("scores", scores[i]+ 1000)
		// row[1] = scores[i] * QueryScore(terms, title)
		row[1] = title
		rows[i] = row
	}
	rows = rows[:len(titles)]

	output := make(map[string]interface{})
	output["terms"] = terms
	output["results"] = rows
	// fmt.Println(output)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	renderer.JSON(w, http.StatusOK, output)
}

var (
	redisPool *pool.Pool
	procs     int
	debug     = true
	renderer  = render.New()
	redisURL  = "127.0.0.1:6379"
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
	flag.IntVar(&redisDatabase, "redisDatabase", redisDatabase,
		"Redis database number to connect to")
	flag.Parse()

	fmt.Println("REDIS DATABASE:", redisDatabase)
	fmt.Println("DEBUG MODE:", debug)

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

	n := negroni.Classic()
	n.UseHandler(mux)
	n.Run(fmt.Sprintf(":%d", port))
}
