package main

import (
	"flag"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	port   = flag.String("port", "8080", "port to listen on")
	auth   = flag.String("auth", "", "Authorization Header of reverse proxy")
	target = flag.String("target", "https://api.openai.com", "ChatGPT API address")
	tokens = flag.String("tokens", "", "comma separated ChatGPT API tokens")
)

var count int64
var lock sync.RWMutex

func main() {
	splits := strings.Split(*tokens, ",")
	var tokens []string
	for _, value := range splits {
		value := strings.Trim(value, " ")
		if len(value) > 0 {
			tokens = append(tokens, value)
		}
	}
	url, err := url.Parse(*target)
	if err != nil {
		panic(err)
	}
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			lock.RLock()
			token := tokens[count%int64(len(tokens))]
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
			return
		}
		proxy.ServeHTTP(w, r)
	})

	err = http.ListenAndServe(":"+*port, nil)
	if err != nil {
		panic(err)
	}
}
