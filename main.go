package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	port       = flag.Int("port", 8080, "Port to listen on")
	auth       = flag.String("auth", "", "Authorization Header of reverse proxy")
	target     = flag.String("target", "https://api.openai.com", "ChatGPT API address")
	tokenstr   = flag.String("tokens", "", "Comma separated ChatGPT API tokens")
	configFile = flag.String("config", "", "The config file path. Config file is prior than commad line")
)

var count int64
var tokens []string

var scheme string
var host string

var lock sync.RWMutex

func main() {
	loadConfig()
	sort.Strings(tokens)
	url, err := url.Parse(*target)
	if err != nil {
		log.Fatalln(err)
	}
	scheme = url.Scheme
	host = url.Host

	proxy := &httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: modifyResponse,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//CORS
		if r.Method == http.MethodOptions {
			w.Header().Add("Access-Control-Allow-Methods", "DELETE,GET,HEAD,OPTIONS,PATCH,POST,PUT")
			w.Header().Add("Access-Control-Allow-Origin", "*")
			w.WriteHeader(200)
			return
		}
		//authorize
		if *auth != "" && *auth != r.Header.Get("Authorization") {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk-") {
				w.WriteHeader(401)
				fmt.Fprint(w, "No Authorization header for proxy server!")
				return
			}
		}
		proxy.ServeHTTP(w, r)
	})

	log.Println("Listen on port:" + strconv.Itoa(*port))
	log.Println("Running...")
	err = http.ListenAndServe(":"+strconv.Itoa(*port), nil)
	if err != nil {
		log.Fatalln(err)
	}
}

func director(req *http.Request) {
	lock.RLock()
	var token string
	if len(tokens) > 0 {
		token = tokens[count%int64(len(tokens))]
	}
	lock.RUnlock()
	atomic.AddInt64(&count, 1)
	req.URL.Scheme = scheme
	req.URL.Host = host
	req.Host = host
	//pass througn
	if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer sk-") {
		req.Header.Del("Authorization")
		if token != "" {
			req.Header.Add("Authorization", "Bearer "+token)
		}
	}
}

func modifyResponse(r *http.Response) error {
	evict := func(r *http.Response) {
		//evict the invalid token
		au := r.Request.Header.Get("Authorization")
		if !strings.HasPrefix(au, "Bearer ") {
			return
		}
		token := strings.Split(au, " ")[1]
		lock.Lock()
		defer lock.Unlock()
		tokens = findAndRemove(tokens, token)
		log.Println("ChatGPT API token " + token + " invalid and has been evicted")
	}

	message429 := struct {
		Error struct {
			Message string      `json:"message"`
			Type    string      `json:"type"`
			Param   interface{} `json:"param"`
			Code    interface{} `json:"code"`
		} `json:"error"`
	}{}

	if r.StatusCode == http.StatusUnauthorized {
		evict(r)
	}
	if r.StatusCode == http.StatusTooManyRequests {
		data, err := io.ReadAll(r.Body)
		if err == nil {
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(data))
			json.Unmarshal(data, &message429)
			if message429.Error.Message == "insufficient_quota" {
				evict(r)
			}
		}
	}
	return nil
}

func findAndRemove(sorted []string, str string) []string {
	i := sort.SearchStrings(sorted, str)
	if i != len(sorted) && str == sorted[i] {
		if i == len(sorted)-1 {
			sorted = sorted[:i]
		} else {
			sorted = append(sorted[:i], sorted[i+1:]...)
		}
	}
	return sorted
}

func loadConfig() {
	type config struct {
		Port   int      `json:"port"`
		Auth   string   `json:"auth"`
		Target string   `json:"target"`
		Tokens []string `json:"tokens"`
	}
	flag.Parse()
	splits := strings.Split(*tokenstr, ",")
	for _, value := range splits {
		value := strings.Trim(value, " ")
		if len(value) > 0 {
			tokens = append(tokens, value)
		}
	}
	if *configFile != "" {
		file, err := os.Open(*configFile)
		if err != nil {
			log.Fatalln(err)
		}
		var config config
		err = json.NewDecoder(file).Decode(&config)
		if err != nil {
			log.Fatalln(err)
		}
		//overwrite command line
		if config.Port != 0 {
			port = &config.Port
		}
		if config.Auth != "" {
			auth = &config.Auth
		}
		if config.Target != "" {
			target = &config.Target
		}
		if len(config.Tokens) > 0 {
			tokens = config.Tokens
		}
	}
}
