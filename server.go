package main

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/deferpanic/deferclient/deferstats"
	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/namsral/flag"
	"github.com/unrolled/render"
	// "github.com/deferpanic/deferclient/errors"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// func QueryScore(terms []string, title) float32 {
// 	return 1.0
// }

func errorHandler(err error) {
	if err != nil {
		log.Panic(err)
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

func getUsername(req *http.Request) string {
	var username string
	cookie, err := req.Cookie("username")
	if err == nil {
		sCookie.Decode("username", cookie.Value, &username)
	}
	return username
}

func indexHandler(w http.ResponseWriter, req *http.Request) {
	context := map[string]interface{}{
		"staticPrefix": staticPrefix,
		"isNotDebug":   !debug,
		"Username":     "",
		"domains":      make([]string, 0),
	}

	username := getUsername(req)
	if username != "" {
		// Yay! You're signed in!
		context["Username"] = username
		c, err := redisPool.Get()
		errorHandler(err)
		defer redisPool.CarefullyPut(c, &err)

		userdomainsKey := fmt.Sprintf("$userdomains$%v", username)
		replies, err := c.Cmd("SMEMBERS", userdomainsKey).List()
		errorHandler(err)

		var domains []domainRow

		var domain string
		for _, key := range replies {
			reply := c.Cmd("HGET", "$domainkeys", key)
			if reply.Type != redis.NilReply {
				domain, err = reply.Str()
				errorHandler(err)
				domains = append(domains, domainRow{
					Key:    key,
					Domain: domain,
				})
			}
		}
		context["domains"] = domains
	}
	// this assumes there's a `templates/index.tmpl` file
	renderer.HTML(w, http.StatusOK, "index", context)
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
		log.Printf(
			"invalid oauth state, expected '%s', got '%s'\n",
			oauthStateString,
			state,
		)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	code := req.FormValue("code")
	token, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	oauthClient := oauthConf.Client(oauth2.NoContext, token)
	client := github.NewClient(oauthClient)
	// the second item here is the github.Rate config
	user, _, err := client.Users.Get("")

	if err != nil {
		log.Printf("client.Users.Get() faled with '%s'\n", err)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	log.Printf("Logged in as GitHub user: %s (%s)\n", *user.Login, *user.Email)
	// fmt.Println(user)
	// store the email
	c, err := redisPool.Get()
	errorHandler(err)
	defer redisPool.CarefullyPut(c, &err)

	err = c.Cmd("SADD", "$usernames", *user.Login).Err
	errorHandler(err)
	err = c.Cmd("HSET", "$emails", *user.Login, *user.Email).Err
	errorHandler(err)

	encoded, err := sCookie.Encode("username", *user.Login)
	errorHandler(err)
	expire := time.Now().AddDate(0, 0, 1) // how long is this?
	secureCookie := isOnHTTPS(req)

	cookie := &http.Cookie{
		Name:     "username",
		Value:    encoded,
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
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func domainkeyNewHandler(w http.ResponseWriter, req *http.Request) {
	domain := strings.Trim(req.FormValue("domain"), " ")
	if domain != "" {
		username := getUsername(req)
		if username != "" {
			c, err := redisPool.Get()
			errorHandler(err)
			defer redisPool.CarefullyPut(c, &err)

			key := randString(24)
			userdomainsKey := fmt.Sprintf("$userdomains$%v", username)
			err = c.Cmd("SADD", userdomainsKey, key).Err
			errorHandler(err)
			err = c.Cmd("HSET", "$domainkeys", key, domain).Err
			errorHandler(err)
		}
	}

	// http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
	http.Redirect(w, req, "/#auth", http.StatusFound)
}

func domainkeyDeleteHandler(w http.ResponseWriter, req *http.Request) {
	key := strings.Trim(req.FormValue("key"), " ")
	if key != "" {
		username := getUsername(req)
		if username != "" {
			// Yay! You're signed in!
			c, err := redisPool.Get()
			errorHandler(err)
			defer redisPool.CarefullyPut(c, &err)

			userdomainsKey := fmt.Sprintf("$userdomains$%v", username)
			err = c.Cmd("SREM", userdomainsKey, key).Err
			errorHandler(err)
			err = c.Cmd("HDEL", "$domainkeys", key).Err
			errorHandler(err)
		} // else, we should yield some sort of 403 message maybe
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
	sCookie      *securecookie.SecureCookie
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
	oauthStateString = randString(24)
)

func main() {
	var (
		port          = 3001
		redisDatabase = 0
		redisPoolSize = 10
		clientID      = ""
		clientSecret  = ""
		hashKey       = "randomishstringthatsi32charslong"
		blockKey      = "randomishstringthatsi32charslong"
		deferPanicKey = ""
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
	flag.StringVar(
		&hashKey, "hashKey", hashKey,
		"HMAC hash key to use for encoding cookies")
	flag.StringVar(
		&blockKey, "blockKey", blockKey,
		"Block key to encrypt cookie values")
	flag.StringVar(
		&deferPanicKey, "deferPanicKey", deferPanicKey,
		"Auth key for deferpanic.com")
	flag.Parse()

	dfs := deferstats.NewClient(deferPanicKey)
	go dfs.CaptureStats()
	// if deferPanicKey != "" {
	// 	dfs := deferstats.NewClient(deferPanicKey)
	// 	go dfs.CaptureStats()
	// }

	oauthConf.ClientID = clientID
	oauthConf.ClientSecret = clientSecret

	sCookie = securecookie.New([]byte(hashKey), []byte(blockKey))

	log.Println("REDIS DATABASE:", redisDatabase)
	log.Println("DEBUG MODE:", debug)
	log.Println("STATIC PREFIX:", staticPrefix)

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
	log.Println("PROCS:", procs)
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
	errorHandler(err)

	mux := mux.NewRouter()
	mux.HandleFunc("/", dfs.HTTPHandler(indexHandler)).Methods("GET", "HEAD")
	mux.HandleFunc("/v1/ping", dfs.HTTPHandler(pingHandler)).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", dfs.HTTPHandler(fetchHandler)).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", dfs.HTTPHandler(updateHandler)).Methods("POST", "PUT")
	mux.HandleFunc("/v1", dfs.HTTPHandler(deleteHandler)).Methods("DELETE")
	mux.HandleFunc("/v1/stats", dfs.HTTPHandler(privateStatsHandler)).Methods("GET")
	mux.HandleFunc("/v1/flush", dfs.HTTPHandler(flushHandler)).Methods("DELETE")
	mux.HandleFunc("/v1/bulk", dfs.HTTPHandler(bulkHandler)).Methods("POST", "PUT")
	mux.HandleFunc("/login", dfs.HTTPHandler(handleGitHubLogin)).Methods("GET")
	mux.HandleFunc("/logout", dfs.HTTPHandler(logoutHandler)).Methods("GET", "POST")
	mux.HandleFunc("/github_oauth_cb", dfs.HTTPHandler(handleGitHubCallback)).Methods("GET")
	mux.HandleFunc("/domainkeys/new", dfs.HTTPHandler(domainkeyNewHandler)).Methods("POST")
	mux.HandleFunc("/domainkeys/delete", dfs.HTTPHandler(domainkeyDeleteHandler)).Methods("POST")

	n := negroni.Classic()

	n.UseHandler(mux)
	n.Run(fmt.Sprintf(":%d", port))
}
