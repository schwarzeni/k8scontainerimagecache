package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var (
	serverAddr    = flag.String("addr", "0.0.0.0:8888", "proxy server address")
	remoteAddr    = flag.String("remote", "https://registry-1.docker.io", "docker remote address")
	nodeList      = []string{"10.211.55.68", "10.211.55.69", "10.211.55.70"} // TODO: 暂时写死，之后从中央节点获取
	currNodeIP    = flag.String("ip", "", "curr node ip")
	dockerRootDir = flag.String("dockerdir", "/var/lib/docker", "docker root dir")
	//remoteAddr = flag.String("remote", "https://4h50gpde.mirror.aliyuncs.com", "docker remote address")
)

func main() {
	flag.Parse()
	if len(*currNodeIP) == 0 {
		panic("curr node ip empty")
	}
	rand.Seed(time.Now().Unix())
	layerMap := LayerMap{dockerImageDataRoot: path.Join(*dockerRootDir, "image")}
	if err := layerMap.UpdateMap(); err != nil {
		panic(err)
	}
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
		if _, ok := layerMap.Cache(layerid); ok {
			respData.HasLayer = true
		}
		respDataBytes, _ := json.Marshal(&respData)
		_, _ = resp.Write(respDataBytes)
	})
	r.HandleFunc("/agentapi/v1/layerdl/{layerid}", func(resp http.ResponseWriter, req *http.Request) {
		layerid := mux.Vars(req)["layerid"]
		cacheid, ok := layerMap.Cache(layerid)
		if !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		layerDataPath := path.Join(*dockerRootDir, "overlay2", cacheid, "diff")
		log.Println("[info] dl layerDataPath", layerDataPath)
		if err := compress(layerDataPath, resp); err != nil {
			log.Printf("[err] read and compress %s failed\n", layerDataPath)
		}
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
					for k, vv := range resp.Header {
						for _, v := range vv {
							w.Header().Add(k, v)
						}
					}
					_, _ = bufio.NewReader(resp.Body).WriteTo(w)
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
