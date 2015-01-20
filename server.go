package main

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/unrolled/render"
	"os"
	// "github.com/gorilla/mux"
	"github.com/fzzy/radix/redis"
	"github.com/fzzy/radix/extra/pool"
	"net/http"
	// "time"
	// "log"
	"strings"
	// "strconv"
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
		IndentJSON: debug,
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

	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		err = client.Cmd("SELECT", 9).Err
		if err != nil {
			return nil, err
		}
		// if err = client.Cmd("AUTH", "SUPERSECRET").Err; err != nil {
		// 	client.Close()
		// 	return nil, err
		// }
		return client, nil
	}
	pool, err := pool.NewCustomPool("tcp", "127.0.0.1:6379", 10, df)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "Welcome to the home page!\n")
	})
	mux.HandleFunc("/v1", func(w http.ResponseWriter, req *http.Request) {
		q := strings.Trim(req.FormValue("q"), " ")
		terms := CleanWords(q)

		c, err := pool.Get()
		errHndlr(err)
		defer pool.Put(c)
		// NOTE! Maybe we don't need the ZINTERSTORE if there's only 1 command
		c.Append("ZINTERSTORE", "$tmp", len(terms), terms, "AGGREGATE", "max")
		c.Append("ZREVRANGE", "$tmp", 0, 10, "WITHSCORES")

		c.GetReply() // the ZINTERSTORE
		replies, err := c.GetReply().List()
		errHndlr(err)

		ooids := make([]string, 10+1)
		scores := make([]string, 10+1)
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

		w.Header().Set("Access-Control-Allow-Origin", "*")
		renderer.JSON(w, http.StatusOK, output)
	})

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
