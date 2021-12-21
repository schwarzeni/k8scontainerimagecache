package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

var (
	authCache     = map[string]string{}
	authCacheLock sync.RWMutex
)

func withDockerhubPullAuth(req *http.Request, imageName string) (err error) {
	// TODO: 后期缓存可以采用 cache + singleflight 优化
	authCacheLock.Lock()
	defer authCacheLock.Unlock()
	token, ok := authCache[imageName]
	if !ok {
		log.Println("[debug] request token for ", imageName)
		resp, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", imageName))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		respObj := dockerhubAuthObj{}
		respData, _ := ioutil.ReadAll(resp.Body)
		if err = json.Unmarshal(respData, &respObj); err != nil {
			return err
		}
		token = respObj.Token
		authCache[imageName] = respObj.Token
	}
	req.Header.Add("Authorization", "Bearer "+token)
	return nil
}

type dockerhubAuthObj struct {
	Token string `json:"token"`
}
