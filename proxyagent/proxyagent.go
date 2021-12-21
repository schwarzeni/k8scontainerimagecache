package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var (
	serverAddr = flag.String("addr", "127.0.0.1:8888", "proxy server address")
	remoteAddr = flag.String("remote", "https://registry-1.docker.io", "docker remote address")
	//remoteAddr = flag.String("remote", "https://4h50gpde.mirror.aliyuncs.com", "docker remote address")
)

func main() {
	flag.Parse()
	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Println("[debug]", r.RequestURI)
			next.ServeHTTP(w, r)
		})
	})
	r.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)
	r.PathPrefix("/v2/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//log.Println("[info]", r.RequestURI)
		urlItems := strings.Split(r.RequestURI, "/")
		imageName := urlItems[2] + "/" + urlItems[3]
		sourceType := urlItems[4]
		sourceID := urlItems[5]
		_ = sourceID
		_ = sourceType
		if err := withDockerhubPullAuth(r, imageName); err != nil {
			log.Println("[err] fetch token for ", imageName, err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp, err := doGetProxy(r)
		if err != nil {
			log.Println("[err]", r.RequestURI, err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		// !!! header 需要 copy
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		_, _ = bufio.NewReader(resp.Body).WriteTo(w)
	}).Methods(http.MethodGet)

	srv := &http.Server{
		Handler:      r,
		Addr:         *serverAddr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func doGetProxy(rawReq *http.Request) (*http.Response, error) {
	targetURL := rawReq.URL.String()
	req, err := http.NewRequest(http.MethodGet, *remoteAddr+targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("newRequest for %s: %v", targetURL, err)
	}
	for k, vv := range rawReq.Header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	client := http.DefaultClient
	client.Timeout = time.Second * 20
	return client.Do(req)
}

//r.HandleFunc("/v2/library/{image}/manifests/{id:(sha256:)?[\\w\\d]+}", func(w http.ResponseWriter, r *http.Request) {
//	imageName := mux.Vars(r)["image"]
//	if err := withDockerhubPullAuth(r, imageName); err != nil {
//		log.Println("fetch token for ", imageName, err)
//		w.WriteHeader(http.StatusServiceUnavailable)
//		return
//	}
//	resp, err := doGetProxy(r)
//	if err != nil {
//		log.Println("do get proxy for ", r.URL.String(), err)
//	}
//	defer resp.Body.Close()
//	bufio.NewReader(resp.Body).WriteTo(w)
//}).Methods(http.MethodGet)
//r.HandleFunc("/v2/library/{image}/blobs/{id:(sha256:)?[\\w\\d]+}", func(w http.ResponseWriter, r *http.Request) {
//	imageName := mux.Vars(r)["image"]
//	if err := withDockerhubPullAuth(r, imageName); err != nil {
//		log.Println("fetch token for ", imageName, err)
//		w.WriteHeader(http.StatusServiceUnavailable)
//		return
//	}
//	resp, err := doGetProxy(r)
//	if err != nil {
//		log.Println("do get proxy for ", r.URL.String(), err)
//	}
//	defer resp.Body.Close()
//	bufio.NewReader(resp.Body).WriteTo(w)
//}).Methods(http.MethodGet)
