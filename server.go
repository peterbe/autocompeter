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
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"time"
)

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
		var username string
		if err = sCookie.Decode("username", cookie.Value, &username); err == nil {
			// Yay! You're signed in!

			context["Username"] = username
			c, err := redisPool.Get()
			errHndlr(err)
			defer redisPool.CarefullyPut(c, &err)

			userdomainsKey := fmt.Sprintf("$userdomains$%v", username)
			replies, err := c.Cmd("SMEMBERS", userdomainsKey).List()
			errHndlr(err)

			var domains []domainRow

			var domain string
			for _, key := range replies {
				reply := c.Cmd("HGET", "$domainkeys", key)
				if reply.Type != redis.NilReply {
					domain, err = reply.Str()
					errHndlr(err)
					domains = append(domains, domainRow{
						Key:    key,
						Domain: domain,
					})
				}
			}
			context["domains"] = domains
		}
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
	// fmt.Printf("Logged in as GitHub user: %s\n", *user)
	encoded, err := sCookie.Encode("username", *user.Login)
	errHndlr(err)
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
			var username string
			if err = sCookie.Decode("username", cookie.Value, &username); err == nil {
				c, err := redisPool.Get()
				errHndlr(err)
				defer redisPool.CarefullyPut(c, &err)

				key := randString(24)
				userdomainsKey := fmt.Sprintf("$userdomains$%v", username)
				err = c.Cmd("SADD", userdomainsKey, key).Err
				errHndlr(err)
				err = c.Cmd("HSET", "$domainkeys", key, domain).Err
				errHndlr(err)
			}
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
			var username string
			if err = sCookie.Decode("username", cookie.Value, &username); err == nil {
				// Yay! You're signed in!
				c, err := redisPool.Get()
				errHndlr(err)
				defer redisPool.CarefullyPut(c, &err)

				userdomainsKey := fmt.Sprintf("$userdomains$%v", username)
				err = c.Cmd("SREM", userdomainsKey, key).Err
				errHndlr(err)
				err = c.Cmd("HDEL", "$domainkeys", key).Err
				errHndlr(err)
			} // else, we should yield some sort of 403 message maybe

		}
	}

	http.Redirect(w, req, "/#auth", http.StatusFound)
}

// This is just a temporary thing to try out deferpanic.com
// and this can be removed once I know it works.
func errorTestHandler(w http.ResponseWriter, req *http.Request) {
	panic("Something terrible happened")
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

	if deferPanicKey != "" {
		deferstats.Token = deferPanicKey
		go deferstats.CaptureStats()
	}

	oauthConf.ClientID = clientID
	oauthConf.ClientSecret = clientSecret

	sCookie = securecookie.New([]byte(hashKey), []byte(blockKey))

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
	mux.HandleFunc("/", deferstats.HTTPHandler(indexHandler)).Methods("GET", "HEAD")
	mux.HandleFunc("/v1/ping", deferstats.HTTPHandler(pingHandler)).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", deferstats.HTTPHandler(fetchHandler)).Methods("GET", "HEAD")
	mux.HandleFunc("/v1", deferstats.HTTPHandler(updateHandler)).Methods("POST", "PUT")
	mux.HandleFunc("/v1", deferstats.HTTPHandler(deleteHandler)).Methods("DELETE")
	mux.HandleFunc("/v1/stats", deferstats.HTTPHandler(privateStatsHandler)).Methods("GET")
	mux.HandleFunc("/v1/flush", deferstats.HTTPHandler(flushHandler)).Methods("DELETE")
	mux.HandleFunc("/v1/bulk", deferstats.HTTPHandler(bulkHandler)).Methods("POST", "PUT")
	mux.HandleFunc("/login", deferstats.HTTPHandler(handleGitHubLogin)).Methods("GET")
	mux.HandleFunc("/logout", deferstats.HTTPHandler(logoutHandler)).Methods("GET", "POST")
	mux.HandleFunc("/github_oauth_cb", deferstats.HTTPHandler(handleGitHubCallback)).Methods("GET")
	mux.HandleFunc("/domainkeys/new", deferstats.HTTPHandler(domainkeyNewHandler)).Methods("POST")
	mux.HandleFunc("/domainkeys/delete", deferstats.HTTPHandler(domainkeyDeleteHandler)).Methods("POST")
	mux.HandleFunc("/errortest", deferstats.HTTPHandler(errorTestHandler)).Methods("GET")

	n := negroni.Classic()

	n.UseHandler(mux)
	n.Run(fmt.Sprintf(":%d", port))
}
