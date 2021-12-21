package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
)

//func main() {
//	layerMap := LayerMap{}
//	if err := layerMap.UpdateMap(); err != nil {
//		panic(err)
//	}
//	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//		id := r.URL.String()[1:]
//		cacheID := layerMap.layerToCache[id]
//		_, _ = w.Write([]byte(cacheID))
//	})
//	if err := http.ListenAndServe(":8080", nil); err != nil {
//		log.Fatal(err)
//	}
//}

type LayerMap struct {
	diffToLayer         map[string]string
	diffToCache         map[string]string
	layerToCache        map[string]string
	cacheToLayers       map[string][]string
	dockerImageDataRoot string
	lock                sync.RWMutex
}

func (lm *LayerMap) Cache(layerID string) (string, bool) {
	cacheID, ok := lm.layerToCache[layerID]
	return cacheID, ok
}

func (lm *LayerMap) UpdateMap() error {
	lm.lock.Lock()
	defer lm.lock.Unlock()
	lm.diffToLayer = map[string]string{}
	lm.diffToCache = map[string]string{}
	lm.layerToCache = map[string]string{}
	lm.cacheToLayers = map[string][]string{}
	if len(lm.dockerImageDataRoot) == 0 {
		lm.dockerImageDataRoot = "/var/lib/docker/image"
	}
	if err := lm.UpdateDiffToLayerMap(); err != nil {
		return err
	}
	if err := lm.UpdateDiffToCacheMap(); err != nil {
		return err
	}
	if err := lm.UpdateLayerAndCacheMap(); err != nil {
		return err
	}
	return nil
}

// UpdateDiffToLayerMap
// 遍历 /var/lib/docker/image/overlay2/distribution/v2metadata-by-diffid/sha256
// 得到 diff id --> layer id 的映射关系
func (lm *LayerMap) UpdateDiffToLayerMap() error {
	diffToLayerDir := path.Join(lm.dockerImageDataRoot, "overlay2/distribution/v2metadata-by-diffid/sha256/")
	if _, err := os.Stat(diffToLayerDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(diffToLayerDir, func(fpath string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("access %s: %v", fpath, err)
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(fpath)
		if err != nil {
			return fmt.Errorf("read file %s: %v", fpath, err)
		}
		var dataJson []DiffIDToLayerID
		if err := json.Unmarshal(data, &dataJson); err != nil {
			return fmt.Errorf("unmarshal json %s: %v", string(data), err)
		}
		for _, v := range dataJson {
			lm.diffToLayer[path.Base(fpath)] = v.Digest[len("sha256:"):]
		}
		return nil
	})
}

// UpdateDiffToCacheMap
// 遍历 /var/lib/docker/image/overlay2/layerdb/sha256
// 根据其中的 diff id 和 cache id 得到 diff id --> cache id 的映射关系
func (lm *LayerMap) UpdateDiffToCacheMap() error {
	layerDBDir := path.Join(lm.dockerImageDataRoot, "overlay2/layerdb/sha256/")
	if _, err := os.Stat(layerDBDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(layerDBDir, func(dirPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("access %s: %v", dirPath, err)
		}
		if !info.IsDir() || dirPath == layerDBDir {
			return nil
		}
		diffIDByte, err := os.ReadFile(path.Join(dirPath, "diff"))
		if err != nil {
			return fmt.Errorf("access %s: %v", path.Join(dirPath, "diff"), err)
		}
		cacheIDByte, err := os.ReadFile(path.Join(dirPath, "cache-id"))
		if err != nil {
			return fmt.Errorf("access %s: %v", path.Join(dirPath, "cache-id"), err)
		}
		lm.diffToCache[string(diffIDByte[len("sha256:"):])] = string(cacheIDByte)
		return nil
	})
}

// UpdateLayerAndCacheMap
// 根据 UpdateDiffToLayerMap 和 UpdateDiffToCacheMap 可以得到 layer id --> cache id 的映射关系和 cache id --> [layer id] 的双向映射
func (lm *LayerMap) UpdateLayerAndCacheMap() error {
	for diffID, layerID := range lm.diffToLayer {
		cacheID, ok := lm.diffToCache[diffID]
		if !ok {
			//return fmt.Errorf("diffID %s does not have cacheID", diffID)
			log.Println("[info] missing cacheid for diffID: ", diffID)
			continue
		}
		lm.layerToCache[layerID] = cacheID
		lm.cacheToLayers[cacheID] = append(lm.cacheToLayers[cacheID], layerID)
	}
	return nil
}

type DiffIDToLayerID struct {
	Digest string `json:"Digest"`
}
