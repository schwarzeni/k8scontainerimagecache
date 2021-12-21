package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type QueryLayerResponse struct {
	HasLayer   bool       `json:"has_layer"`
	NodeStatus NodeStatus `json:"node_status"`
}

type NodeStatus struct {
}

func query(nodes []string, layerID string) (responses []*QueryLayerResponse) {
	timeout := time.Millisecond * 100
	reschan := make(chan *QueryLayerResponse)
	client := http.Client{Timeout: timeout}
	for _, node := range nodes {
		go func(node string) {
			apiURL := "http://" + node + ":8888/agentapi/v1/layerquery/" + layerID
			resp, err := client.Get(apiURL)
			if err != nil {
				log.Println("[info] err, access url ", apiURL, err)
				// just ignore
				reschan <- nil
				return
			}
			defer resp.Body.Close()
			respData := QueryLayerResponse{}
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println("[info] err, read resp from ", apiURL, err)
				// just ignore it
				reschan <- nil
				return
			}
			_ = json.Unmarshal(bodyBytes, &respData)
			reschan <- &respData
		}(node)
	}
	for i := 0; i < len(nodes); i++ {
		res := <-reschan
		if res != nil && res.HasLayer {
			responses = append(responses, res)
		}
	}
	return responses
}

func main() {

}
