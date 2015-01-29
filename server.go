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

func IndexHandler(w http.ResponseWriter, req *http.Request) {
	// this assumes there's a `templates/index.tmpl` file
	renderer.HTML(w, http.StatusOK, "index", nil)
}

type UpdateForm struct {
	Domain     string
	Url        string
	Title      string
	Groups     string
	Popularity float64
}

func (f *UpdateForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.Domain: binding.Field{
			Form:     "domain",
			Required: true,
		},
		&f.Url: binding.Field{
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

func (f UpdateForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
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
	if strings.Trim(f.Url, " ") == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"url"},
			Classification: "ComplaintError",
			Message:        "Can't be empty",
		})
	}
	return errs
}

func UpdateHandler(w http.ResponseWriter, req *http.Request) {
	form := new(UpdateForm)
	errs := binding.Bind(req, form)
	if errs.Handle(w) {
		return
	}
	form.Domain = strings.Trim(form.Domain, " ")
	form.Title = strings.Trim(form.Title, " ")
	form.Url = strings.Trim(form.Url, " ")

	encoded := encodeString(form.Domain)
	url_encoded := encodeString(form.Url)

	c, err := redis_pool.Get()
	errHndlr(err)
	defer redis_pool.Put(c)
	// c.Cmd("FLUSHALL")
	// fmt.Println("CAREFUL! Always flushing the database")
	piped_commands := 0
	for _, prefix := range getPrefixes(form.Title) {
		c.Append("ZADD", encoded+prefix, form.Popularity, url_encoded)
		piped_commands += 1
	}
	c.Append("HSET", encoded+"$titles", url_encoded, form.Title)
	piped_commands += 1
	c.Append("HSET", encoded+"$urls", url_encoded, form.Url)
	piped_commands += 1
	for i := 1; i <= piped_commands; i++ {
		if err := c.GetReply().Err; err != nil {
			errHndlr(err)
		}
	}

	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusCreated, output)
}

type FetchForm struct {
	Number int
	Query  string
	Domain string
}

func (f *FetchForm) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&f.Number: "n",
		&f.Query: binding.Field{
			Form:     "q",
			Required: true,
		},
		&f.Domain: binding.Field{
			Form:     "domain",
			Required: true,
		},
	}
}

func FetchHandler(w http.ResponseWriter, req *http.Request) {
	form := new(FetchForm)
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

	c, err := redis_pool.Get()
	errHndlr(err)
	defer redis_pool.CarefullyPut(c, &err)

	encoded_terms := make([]string, len(terms))
	for i, term := range terms {
		encoded_terms[i] = encoded + term
	}
	// NOTE! Maybe we don't need the ZINTERSTORE if there's only 1 command
	c.Append("ZINTERSTORE", "$tmp", len(terms), encoded_terms, "AGGREGATE", "max")
	c.Append("ZREVRANGE", "$tmp", 0, n-1, "WITHSCORES")

	c.GetReply() // the ZINTERSTORE
	replies, err := c.GetReply().List()
	// fmt.Println("replies", replies, len(replies))
	errHndlr(err)

	encoded_urls := make([]string, n+1)
	scores := make([]string, n+1)
	evens := 0
	for i, element := range replies {
		if i%2 == 0 {
			encoded_urls[evens] = element
			evens = evens + 1
		} else {
			scores[evens-1] = element
		}
	}
	encoded_urls = encoded_urls[:evens]
	scores = scores[:evens]

	var titles []string
	var urls []string
	if len(encoded_urls) == 0 {
	} else {
		titles, err = c.Cmd("HMGET", encoded+"$titles", encoded_urls).List()
		errHndlr(err)
		urls, err = c.Cmd("HMGET", encoded+"$urls", encoded_urls).List()
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
	redis_pool *pool.Pool
	procs      int
	debug      = true
	renderer   = render.New()
	redis_url  = "127.0.0.1:6379"
)

func main() {
	var port = 3001
	// var redis_url = "127.0.0.1:6379"
	var redis_database = 0
	flag.IntVar(&port, "port", port, "Port to start the server on")
	flag.IntVar(&procs, "procs", 1, "Number of CPU processors (0 to use max)")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.StringVar(
		&redis_url, "redis_url", redis_url,
		"Redis URL to tcp connect to")
	flag.IntVar(&redis_database, "redis_database", redis_database,
		"Redis database number to connect to")
	flag.Parse()

	// Figuring out how many processors to use.
	max_procs := runtime.NumCPU()
	if procs == 0 {
		procs = max_procs
	} else if procs < 0 {
		panic("PROCS < 0")
	} else if procs > max_procs {
		panic(fmt.Sprintf("PROCS > max (%v)", max_procs))
	}
	// fmt.Println("procs=", procs)
	fmt.Printf("Running on %d procs\n", procs)
	runtime.GOMAXPROCS(procs)

	fmt.Println("DEBUG MODE:", debug)
	renderer = render.New(render.Options{
		IndentJSON:    debug,
		IsDevelopment: debug,
	})

	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		// fmt.Println("DIaling")
		if err != nil {
			return nil, err
		}
		err = client.Cmd("SELECT", redis_database).Err
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
	// fmt.Println("redis_url:", redis_url)
	// fmt.Println("redis_database:", redis_database)
	// fmt.Println("pool size", procs*10)
	redis_pool, err = pool.NewCustomPool("tcp", redis_url, 100, df)
	errHndlr(err)

	mux := mux.NewRouter()
	mux.HandleFunc("/", IndexHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", FetchHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", UpdateHandler).Methods("POST", "PUT")

	n := negroni.Classic()
	n.UseHandler(mux)
	n.Run(fmt.Sprintf(":%d", port))
}
