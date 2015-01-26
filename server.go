package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/gorilla/mux"
	"github.com/unrolled/render"
	"net/http"
	"os"
	"runtime"
	// "io"
	// "time"
	// "log"
	"strconv"
	"strings"
	// "regexp"
)

func cleanWords(query string) []string {
	// r := regexp.MustCompile(regexp.QuoteMeta("[]()"))
	// query = r.ReplaceAllString(query, " ")
	query = strings.Replace(query, "\"", " ", -1)
	query = strings.Replace(query, "[", " ", -1)
	query = strings.Replace(query, "]", " ", -1)
	query = strings.Replace(query, "(", " ", -1)
	query = strings.Replace(query, ")", " ", -1)
	query = strings.Replace(query, "?", " ", -1)
	query = strings.Replace(query, "!", " ", -1)
	query = strings.Replace(query, ",", " ", -1)
	query = strings.Replace(query, "-", " ", -1)
	query = strings.Replace(query, ":", " ", -1)
	query = strings.Replace(query, ";", " ", -1)
	query = strings.Replace(query, ",", " ", -1)
	query = strings.Replace(query, "'", " ", -1)
	split := strings.Fields(strings.Trim(query, " "))
	terms := make([]string, len(split))
	for i, term := range split {
		terms[i] = strings.ToLower(strings.Trim(strings.Trim(term, " "), "."))
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
		os.Exit(1)
	}
}

func IndexHandler(w http.ResponseWriter, req *http.Request) {
	// this assumes there's a `templates/index.tmpl` file
	renderer.HTML(w, http.StatusOK, "index", nil)
}

/* POSTING new stuff in */
func UpdateHandler(w http.ResponseWriter, req *http.Request) {
	values := make(map[string]string)

	required := []string{"domain", "url", "title"}
	errors := make(map[string]string)
	var value string
	for _, key := range required {
		value = strings.Trim(req.FormValue(key), " ")
		values[key] = value
		if value == "" {
			errors[key] = "Missing"
		}
	}

	optional := []string{"groups", "popularity"}
	for _, key := range optional {
		value = strings.Trim(req.FormValue(key), " ")
		if key == "groups" {
			// values[key] = strings.Split(value, ",")
			values[key] = value
		} else if key == "popularity" {
			if value == "" {
				values[key] = "0"
			} else {
				_, err := strconv.ParseFloat(value, 0)
				if err != nil {
					errors[key] = "Not a number"
				} else {
					values[key] = value
				}
			}
		} else {
			values[key] = value
		}
	}
	// fmt.Println("errors=", errors, len(errors))
	if len(errors) > 0 {
		error := make(map[string]interface{})
		error["error"] = errors
		renderer.JSON(w, http.StatusBadRequest, error)
		return
	}
	// popularity, _ := strconv.ParseInt(values["popularity"], 10, 0)
	popularity, _ := strconv.ParseFloat(values["popularity"], 0)

	encoded := encodeString(values["domain"])
	values["url_encoded"] = encodeString(values["url"])

	c, err := redis_pool.Get()
	errHndlr(err)
	defer redis_pool.Put(c)
	// c.Cmd("FLUSHALL")
	// fmt.Println("CAREFUL! Always flushing the database")
	piped_commands := 0
	for _, prefix := range getPrefixes(values["title"]) {
		c.Append("ZADD", encoded+prefix, popularity, values["url_encoded"])
		piped_commands += 1
	}
	c.Append("HSET", encoded+"$titles", values["url_encoded"], values["title"])
	piped_commands += 1
	c.Append("HSET", encoded+"$urls", values["url_encoded"], values["url"])
	piped_commands += 1
	for i := 1; i <= piped_commands; i++ {
		if err := c.GetReply().Err; err != nil {
			errHndlr(err)
		}
	}

	output := map[string]string{"message": "OK"}
	renderer.JSON(w, http.StatusCreated, output)
}

func FetchHandler(w http.ResponseWriter, req *http.Request) {

	n_str := req.FormValue("n")

	var n int64
	if n_str == "" {
		n = 10
	} else {
		n, err := strconv.ParseInt(n_str, 10, 0)
		if err != nil {
			// error := make(map[string]string{"error": "Not a number"})
			error := map[string]string{"error": "Not a number"}
			renderer.JSON(w, http.StatusBadRequest, error)
			return
		}
		if n <= 0 {
			error := map[string]string{"error": "Number too small"}
			renderer.JSON(w, http.StatusBadRequest, error)
			return
		}
		if n > 100 {
			error := map[string]string{"error": "Number too big"}
			renderer.JSON(w, http.StatusBadRequest, error)
			return
		}
	}

	domain := strings.Trim(req.FormValue("domain"), " ")
	if domain == "" {
		error := map[string]string{"error": "'domain' missing"}
		renderer.JSON(w, http.StatusBadRequest, error)
		return
	}

	encoded := encodeString(domain)
	// fmt.Println(domain, encoded)

	q := strings.Trim(req.FormValue("q"), " ")
	terms := cleanWords(q)

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
	renderer   *render.Render
	redis_pool *pool.Pool
	procs      int64 // because that's what ParseInt() gives us
	debug      = true
)

func main() {
	// Figuring out how many processors to use.
	procs_str := os.Getenv("PROCS")
	max_procs := int64(runtime.NumCPU())
	if procs_str == "" {
		procs_str = "1"
	}
	var err error
	if procs_str == "0" {
		procs = int64(runtime.NumCPU())
	} else {
		procs, err = strconv.ParseInt(procs_str, 10, 0)
		errHndlr(err)
	}
	if procs < 0 {
		panic("PROCS < 0")
	} else if procs > max_procs {
		panic(fmt.Sprintf("PROCS > max (%v)", max_procs))
	}
	runtime.GOMAXPROCS(int(procs))

	// Figuring out if this is debug mode or not.
	if os.Getenv("DEBUG") == "" ||
		os.Getenv("DEBUG") == "false" ||
		os.Getenv("DEBUG") == "0" {
		debug = false
	}

	renderer = render.New(render.Options{
		IndentJSON:    debug,
		IsDevelopment: debug,
	})

	REDIS_URL := os.Getenv("REDIS_URL")
	if REDIS_URL == "" {
		REDIS_URL = "127.0.0.1:6379"
	}
	REDIS_DATABASE := os.Getenv("REDIS_DATABASE")
	if REDIS_DATABASE == "" {
		REDIS_DATABASE = "0"
	}

	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		// fmt.Println("DIaling")
		if err != nil {
			return nil, err
		}
		err = client.Cmd("SELECT", REDIS_DATABASE).Err
		if err != nil {
			return nil, err
		}
		// if err = client.Cmd("AUTH", "SUPERSECRET").Err; err != nil {
		// 	client.Close()
		// 	return nil, err
		// }
		return client, nil
	}

	redis_pool, err = pool.NewCustomPool("tcp", REDIS_URL, int(procs)*10, df)
	errHndlr(err)

	mux := mux.NewRouter()
	mux.HandleFunc("/", IndexHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", FetchHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", UpdateHandler).Methods("POST", "PUT")

	n := negroni.Classic()
	n.UseHandler(mux)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	n.Run(":" + port)
}
