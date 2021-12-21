package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var (
	serverAddr = flag.String("addr", "0.0.0.0:8888", "proxy server address")
	remoteAddr = flag.String("remote", "https://registry-1.docker.io", "docker remote address")
	//nodeList   = []string{"10.211.55.69", "10.211.55.70"} // TODO: 暂时写死，之后从中央节点获取
	nodeList      = []string{"10.211.55.68", "10.211.55.69", "10.211.55.70"} // TODO: 暂时写死，之后从中央节点获取
	currNodeIP    = flag.String("ip", "", "curr node ip")
	dockerRootDir = flag.String("dockerdir", "/var/lib/docker", "docker root dir")
	cacheFolder   = flag.String("cachefolder", "/root/cache", "cache folder")
	//remoteAddr = flag.String("remote", "https://4h50gpde.mirror.aliyuncs.com", "docker remote address")
)

func main() {
	flag.Parse()
	if len(*currNodeIP) == 0 {
		panic("curr node ip empty")
	}
	rand.Seed(time.Now().Unix())
	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Println("[debug]", r.RequestURI)
			next.ServeHTTP(w, r)
		})
	})
	r.HandleFunc("/agentapi/v1/layerquery/{layerid}", func(resp http.ResponseWriter, req *http.Request) {
		respData := QueryLayerResponse{NodeIP: *currNodeIP}
		layerid := mux.Vars(req)["layerid"]
		if _, err := os.Stat(path.Join(*cacheFolder, layerid)); !os.IsNotExist(err) {
			respData.HasLayer = true
		}
		respDataBytes, _ := json.Marshal(&respData)
		_, _ = resp.Write(respDataBytes)
	})
	r.HandleFunc("/agentapi/v1/layerdl/{layerid}", func(resp http.ResponseWriter, req *http.Request) {
		layerid := mux.Vars(req)["layerid"]
		if _, err := os.Stat(path.Join(*cacheFolder, layerid)); os.IsNotExist(err) {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		f, err := os.Open(path.Join(*cacheFolder, layerid))
		if err != nil {
			log.Printf("[err] failed to open %s: %v\n", path.Join(*cacheFolder, layerid), err)
		}
		defer f.Close()
		_, _ = io.Copy(resp, f)
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
		var writer io.Writer = w
		if sourceType == "blobs" {
			layerID := sourceID[len("sha256:"):]
			cacheFile := path.Join(*cacheFolder, layerID)
			f, err := os.OpenFile(cacheFile, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				log.Printf("[err] create file %s: %v\n", cacheFile, err)
			}
			defer f.Close()
			writer = io.MultiWriter(w, f)
		}

		if sourceType == "blobs" {
			layerID := sourceID[len("sha256:"):]
			responses := queryForLayer(nodeList, layerID)
			// TODO: 这里先忽略决策，随机选一个
			if len(responses) > 0 {
				targetNode := responses[rand.Intn(len(responses))]
				resp, err := layerdl(targetNode.NodeIP, layerID)
				if err == nil {
					defer resp.Body.Close()
					log.Printf("[info] dl layer %s from %s\n", layerID, targetNode.NodeIP)
					_ = withDockerhubPullAuth(r, imageName)
					// TODO: need get the header, 否则无效，不清楚哪些 header 起作用
					tmpResp, _ := doGetProxy(r)
					for k, vv := range tmpResp.Header {
						for _, v := range vv {
							w.Header().Add(k, v)
							log.Println("[debug] header: ", k, v)
						}
					}
					tmpResp.Body.Close()
					if _, err = bufio.NewReader(resp.Body).WriteTo(writer); err != nil {
						//panic(err)
					}
					return
				}
				log.Printf("[info] dl layer %s from %s has err: %v, using dockerhub instead\n", layerID, targetNode.NodeIP, err)
			}
		}

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
		if _, err = bufio.NewReader(resp.Body).WriteTo(writer); err != nil {
			//panic(err)
		}
	}).Methods(http.MethodGet)

	srv := &http.Server{
		Handler:      r,
		Addr:         *serverAddr,
		WriteTimeout: 10000 * time.Second,
		ReadTimeout:  10000 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func doGetProxy(rawReq *http.Request) (*http.Response, error) {
	targetURL := rawReq.URL.String()
	os.Setenv("HTTP_PROXY", "http://10.211.55.2:7890")
	os.Setenv("HTTPS_PROXY", "http://10.211.55.2:7890")
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
	client.Timeout = time.Second * 100000
	return client.Do(req)
}
