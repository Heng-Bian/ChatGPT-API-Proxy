package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	port       = flag.String("port", "8080", "Port to listen on")
	auth       = flag.String("auth", "", "Authorization Header of reverse proxy")
	target     = flag.String("target", "https://api.openai.com", "ChatGPT API address")
	tokenstr   = flag.String("tokens", "", "Comma separated ChatGPT API tokens")
	configFile = flag.String("config", "", "The config file path. Config file is prior than commad line")
)

var count int64
var lock sync.RWMutex
var tokens []string

func main() {
	loadConfig()
	url, err := url.Parse(*target)
	if err != nil {
		panic(err)
	}
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			lock.RLock()
			var token string
			if len(tokens) > 0 {
				token = tokens[count%int64(len(tokens))]
			}
			lock.RUnlock()
			atomic.AddInt64(&count, 1)
			req.URL.Scheme = url.Scheme
			req.URL.Host = url.Host
			req.Host = url.Host
			req.Header.Del("Authorization")
			req.Header.Add("Authorization", "Bearer "+token)
		},
		ModifyResponse: func(r *http.Response) error {
			if r.StatusCode != 401 {
				return nil
			}
			au := r.Request.Header.Get("Authorization")
			if strings.HasPrefix(au, "Bearer ") {
				token := strings.Split(au, " ")[1]
				lock.Lock()
				defer lock.Unlock()
				for i, value := range tokens {
					if token == value {
						//end of the slice
						if i == len(tokens)-1 {
							tokens = tokens[:i]
						} else {
							tokens = append(tokens[:i], tokens[i+1:]...)
						}
						log.Println("ChatGPT API token " + token + " invalid and has been evicted")
						break
					}
				}
			}
			return nil
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if *auth != "" && *auth != r.Header.Get("Authorization") {
			w.WriteHeader(401)
			fmt.Fprint(w, "No Authorization header for proxy server!")
			return
		}
		proxy.ServeHTTP(w, r)
	})

	log.Println("Listen on port:" + *port)
	log.Println("Running...")

	err = http.ListenAndServe(":"+*port, nil)
	if err != nil {
		panic(err)
	}
}

func loadConfig() {
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
		configMap := map[string]interface{}{}
		err = json.NewDecoder(file).Decode(&configMap)
		if err != nil {
			log.Fatalln(err)
		}
		if po := configMap["port"]; po != nil {
			poValue, ok := po.(string)
			if ok {
				port = &poValue
			} else {
				log.Fatalln("port in config.json must be string")
			}
		}
		if au := configMap["auth"]; au != nil {
			auValue, ok := au.(string)
			if ok {
				auth = &auValue
			}
		}
		if ta := configMap["target"]; ta != nil {
			taValue, ok := ta.(string)
			if ok {
				target = &taValue
			}
		}
		if to := configMap["tokens"]; to != nil {
			toValue, ok := to.([]string)
			if ok {
				tokens = toValue
			}
		}
	}
}
