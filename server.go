package main

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/unrolled/render"
	"os"
	"github.com/gorilla/mux"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"net/http"
	// "time"
	// "log"
	"strconv"
	"strings"
	// "regexp"
)

func CleanWords(query string) []string {
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

func getPrefixes(title string) []string {
	var prefixes []string
	for _, word := range CleanWords(title) {
		// fmt.Println("  word=", word)
		for i := 1; i <= len(word); i++ {
		// for i := range len(word) {
			prefixes = append(prefixes, word[0:i])
			// fmt.Println("    w=", word[0:i])
		}
		// prefixes = append(prefixes, word)
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

func main() {
	is_debug := os.Getenv("DEBUG")
	// fmt.Println("debug", is_debug)
	var debug = true
	if is_debug == "" {
		debug = false
	}
	renderer := render.New(render.Options{
		IndentJSON:    debug,
		IsDevelopment: debug,
	})

	// config := goredis.DialConfig {
	// 	Network:  "tcp",
	// 	Address:  "127.0.0.1:6379",
	// 	Database: 9,
	// 	Password: "",
	// 	Timeout:  10*time.Second,
	// 	MaxIdle:  10,
	// }

	// c, err := goredis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
	// c, err := goredis.Dial(&config)
	// errHndlr(err)
	// defer client.Close()

	// select database
	// r := c.Cmd("select", 9)
	// errHndlr(r.Err)

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

	pool, err := pool.NewCustomPool("tcp", REDIS_URL, 10, df)
	errHndlr(err)

	// c, err := redis.Dial("tcp", "localhost:6379")
	// errHndlr(err)
	// r := c.Cmd("select", 9)
	// errHndlr(r.Err)

	// // Using redigo
	// c, err := redis.Dial("tcp", ":6379")
	// if err != nil {
	// 	// handle error
	// }
	// defer c.Close()
	// c.Do("SELECT", 9)

	// mux := http.NewServeMux()
	HomeHandler := func(w http.ResponseWriter, req *http.Request) {

		//}

		//mux.HandleFunc("/v1", func(w http.ResponseWriter, req *http.Request) {
		// n_str :=
		n_str := req.FormValue("n")

		var n int64
		if n_str == "" {
			n = 10
		} else {
			n, err = strconv.ParseInt(n_str, 10, 0)
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

		q := strings.Trim(req.FormValue("q"), " ")
		terms := CleanWords(q)

		c, err := pool.Get()
		errHndlr(err)
		defer pool.Put(c)
		// NOTE! Maybe we don't need the ZINTERSTORE if there's only 1 command
		c.Append("ZINTERSTORE", "$tmp", len(terms), terms, "AGGREGATE", "max")
		c.Append("ZREVRANGE", "$tmp", 0, n-1, "WITHSCORES")

		c.GetReply() // the ZINTERSTORE
		replies, err := c.GetReply().List()
		// fmt.Println("replies", replies, len(replies))
		errHndlr(err)

		ooids := make([]string, n+1)
		scores := make([]string, n+1)
		evens := 0
		for i, element := range replies {
			if i%2 == 0 {
				ooids[evens] = element
				evens = evens + 1
			} else {
				scores[evens-1] = element
			}
		}
		ooids = ooids[:evens]
		scores = scores[:evens]

		var titles []string
		if len(ooids) == 0 {
			// titles = [1]string{}
		} else {
			titles, err = c.Cmd("HMGET", "$titles", ooids).List()
			errHndlr(err)

		}
		rows := make([]interface{}, len(titles))
		for i, title := range titles {
			row := make([]string, 2)
			row[0] = ooids[i]
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

	/* POSTING new stuff in */
	UpdateHandler := func(w http.ResponseWriter, req *http.Request) {
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

		c, err := pool.Get()
		errHndlr(err)
		defer pool.Put(c)
		// c.Cmd("FLUSHALL")
		// fmt.Println("CAREFUL! Always flushing the database")
		piped_commands := 0
		for _, prefix := range getPrefixes(values["title"]) {
			// fmt.Println("prefix=", prefix)
			c.Append("ZADD", prefix, popularity, values["url"])
			piped_commands += 1
		}
		c.Append("HSET", "$titles", values["url"], values["title"])
		piped_commands += 1
		// for i <= piped_commands {
		for i := 1; i <= piped_commands; i++ {
			// i += 1
			if err := c.GetReply().Err; err != nil {
				errHndlr(err)
			}
		}
		// domain := req.FormValue("domain")
		// url := req.FormValue("url")
		// id := req.FormValue("id")
		// title := req.FormValue("title")
		// groups := req.FormValue("groups")

		output := map[string]string{"message": "OK"}
		renderer.JSON(w, http.StatusCreated, output)
	}

	mux := mux.NewRouter()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// this assumes there's a `templates/index.tmpl` file
		renderer.HTML(w, http.StatusOK, "index", nil)
	})
	mux.HandleFunc("/v1", HomeHandler).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", UpdateHandler).Methods("POST", "PUT")

	// router := mux.NewRouter()
	// router.HandleFunc("/", HomeHandler)

	n := negroni.Classic()
	n.UseHandler(mux)
	// n.UseHandler(router)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	n.Run(":" + port)
}
