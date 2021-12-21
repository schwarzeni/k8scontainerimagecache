package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

var (
	serverAddr = flag.String("addr", "127.0.0.1:8888", "proxy server address")
	remoteAddr = flag.String("remote", "https://registry-1.docker.io", "docker remote address")
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("access url:", r.URL.String())
	if r.URL.String() == "/v2/" {
		w.WriteHeader(http.StatusOK)
		return
	}

	req, err := http.NewRequest(http.MethodGet, *remoteAddr+r.URL.String(), nil)
	if err != nil {
		panic(err)
	}
	for k, vv := range r.Header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	fmt.Println("======", r.URL.String())
	if err := withDockerhubPullAuth(req, strings.Split(r.URL.String(), "/")[3]); err != nil {
		panic(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	fmt.Println(resp.StatusCode)
	if r.URL.String() == "/v2/" {
		v2data, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("/v2/ resp: ", string(v2data))
		_, _ = fmt.Fprint(w, string(v2data))
		return
	}
	bufio.NewReader(resp.Body).WriteTo(w)

	//step 1 解析代理地址，并更改请求体的协议和主机
	//proxy, err := url.Parse(*remoteAddr)
	//r.URL.Scheme = proxy.Scheme
	//r.URL.Host = proxy.Host
	////step 2 请求下游
	//transport := http.DefaultTransport
	//resp, err := transport.RoundTrip(r)
	//if err != nil {
	//	log.Print(err)
	//	return
	//}
	//
	////step 3 把下游请求内容返回给上游
	//for k, vv := range resp.Header {
	//	for _, v := range vv {
	//		w.Header().Add(k, v)
	//	}
	//}
	//defer resp.Body.Close()
	//fmt.Println(resp.StatusCode)
	//bufio.NewReader(resp.Body).WriteTo(w)
	//data, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println("data for", r.URL.String(), string(data), resp.Status)
	//bytes.NewBuffer(data).WriteTo(w)
}

func main() {
	http.HandleFunc("/", handler)
	err := http.ListenAndServe(*serverAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

//
//func NewSingleHostReverseProxy(target *url.URL) *httputil.ReverseProxy {
//	targetQuery := target.RawQuery
//	director := func(req *http.Request) {
//		for name, values := range req.Header {
//			// Loop over all values for the name.
//			for _, value := range values {
//				fmt.Println(name, value)
//			}
//		}
//		req.URL.Scheme = target.Scheme
//		req.URL.Host = target.Host
//		req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
//		if targetQuery == "" || req.URL.RawQuery == "" {
//			req.URL.RawQuery = targetQuery + req.URL.RawQuery
//		} else {
//			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
//		}
//		if _, ok := req.Header["User-Agent"]; !ok {
//			// explicitly disable User-Agent so it's not set to default value
//			req.Header.Set("User-Agent", "")
//		}
//		log.Println(req.URL.String())
//	}
//	return &httputil.ReverseProxy{
//		Director: director,
//		ModifyResponse: func(response *http.Response) error {
//			data, _ := ioutil.ReadAll(response.Body)
//			log.Println("response ...", string(data), response.Status)
//			return nil
//		},
//	}
//}
//
//func main() {
//	url1, err1 := url.Parse(*remoteAddr)
//	if err1 != nil {
//		log.Println(err1)
//	}
//	proxy := NewSingleHostReverseProxy(url1)
//	log.Println("Starting httpserver at " + *serverAddr)
//	log.Fatal(http.ListenAndServe(*serverAddr, proxy))
//}
//
//func joinURLPath(a, b *url.URL) (path, rawpath string) {
//	if a.RawPath == "" && b.RawPath == "" {
//		return singleJoiningSlash(a.Path, b.Path), ""
//	}
//	// Same as singleJoiningSlash, but uses EscapedPath to determine
//	// whether a slash should be added
//	apath := a.EscapedPath()
//	bpath := b.EscapedPath()
//
//	aslash := strings.HasSuffix(apath, "/")
//	bslash := strings.HasPrefix(bpath, "/")
//
//	switch {
//	case aslash && bslash:
//		return a.Path + b.Path[1:], apath + bpath[1:]
//	case !aslash && !bslash:
//		return a.Path + "/" + b.Path, apath + "/" + bpath
//	}
//	return a.Path + b.Path, apath + bpath
//}
//
//func singleJoiningSlash(a, b string) string {
//	aslash := strings.HasSuffix(a, "/")
//	bslash := strings.HasPrefix(b, "/")
//	switch {
//	case aslash && bslash:
//		return a + b[1:]
//	case !aslash && !bslash:
//		return a + "/" + b
//	}
//	return a + b
//}

const token = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsIng1YyI6WyJNSUlDK1RDQ0FwK2dBd0lCQWdJQkFEQUtCZ2dxaGtqT1BRUURBakJHTVVRd1FnWURWUVFERXp0U1RVbEdPbEZNUmpRNlEwZFFNenBSTWtWYU9sRklSRUk2VkVkRlZUcFZTRlZNT2taTVZqUTZSMGRXV2pwQk5WUkhPbFJMTkZNNlVVeElTVEFlRncweU1UQXhNalV5TXpFMU1EQmFGdzB5TWpBeE1qVXlNekUxTURCYU1FWXhSREJDQmdOVkJBTVRPMVZQU1ZJNlJFMUpWVHBZVlZKUk9rdFdRVXc2U2twTFZ6cExORkpGT2tWT1RFczZRMWRGVERwRVNrOUlPbEpYTjFjNlRrUktWRHBWV0U1WU1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBbnZBeVpFK09sZHgrY3hRS0RBWUtmTHJJYk5rK2hnaEg3Ti9mTFpMVDhEYXVPMXRoTWdoamxjcGhFVkNuYTFlMEZpOHVsUlZ4WG1HdWpZVDNXbnFsZ2ZpM2ZYTUQvQlBRTmlkWHZkeWprbDFZS3dPTkl3TkFWMnRXbExxaXFsdGhSWkFnTFdvWWZZMXZQMHFKTFZBbWt5bUkrOXRBcEMxNldNZ1ZFcHJGdE1rNnV0NDlMcDlUR1J0aDJQbHVWc3RSQ1hVUGp4bjI0d3NnYlUwVStjWTJSNEpyZmVJdzN0T1ZKbXNESkNaYW5SNmVheFYyVFZFUkxoZnNGVTlsSHAzcldCZ1RuNVRCSHlMRDNRdGVFLzJ3L3MvcUxZcmdIK1hCMmZBazJPd1NIRG5YWDg4WWVJd0EyVGJJMDdYNS8xQnVsaUwrUDduOWVBT1RmbDkxVlZwNER3SURBUUFCbzRHeU1JR3ZNQTRHQTFVZER3RUIvd1FFQXdJSGdEQVBCZ05WSFNVRUNEQUdCZ1JWSFNVQU1FUUdBMVVkRGdROUJEdFZUMGxTT2tSTlNWVTZXRlZTVVRwTFZrRk1Pa3BLUzFjNlN6UlNSVHBGVGt4TE9rTlhSVXc2UkVwUFNEcFNWemRYT2s1RVNsUTZWVmhPV0RCR0JnTlZIU01FUHpBOWdEdFNUVWxHT2xGTVJqUTZRMGRRTXpwUk1rVmFPbEZJUkVJNlZFZEZWVHBWU0ZWTU9rWk1WalE2UjBkV1dqcEJOVlJIT2xSTE5GTTZVVXhJU1RBS0JnZ3Foa2pPUFFRREFnTklBREJGQWlFQTBkN3l1azQrWElabmtQb3RJVkdCeHBRSndpMzQwdExSb3R3Qzl4NkJpdWNDSUhFSmIyWGg0QzhtYVZic1Exd3ZUSCthRGV0VXhBS21lYkdXa3F6Z1J1Z1QiXX0.eyJhY2Nlc3MiOlt7InR5cGUiOiJyZXBvc2l0b3J5IiwibmFtZSI6ImxpYnJhcnkvbmdpbngiLCJhY3Rpb25zIjpbInB1bGwiXSwicGFyYW1ldGVycyI6eyJwdWxsX2xpbWl0IjoiMTAwIiwicHVsbF9saW1pdF9pbnRlcnZhbCI6IjIxNjAwIn19XSwiYXVkIjoicmVnaXN0cnkuZG9ja2VyLmlvIiwiZXhwIjoxNjM1MTUzMjk1LCJpYXQiOjE2MzUxNTI5OTUsImlzcyI6ImF1dGguZG9ja2VyLmlvIiwianRpIjoiNnZ2MEx1LXZqU1dHNDQtNEhFM0MiLCJuYmYiOjE2MzUxNTI2OTUsInN1YiI6IiJ9.hVtRR-nKOZBpadrrCJ-NyhnRV4AsRMAkl_g00Fj76R_VlfEHbeuqa9_gzvzS_R2Csa8UBb2GagYtryjGpqPGkrJoU6u6dNxVBH9Op4z1L4AVicStE0cvlJZxof61NrzFclbtg9zekT67bBw-wJdm8Kx4jp0R45pKkYSv1BlNcHQM7EsChIw65dxuiHR9jxW2ehg8muBe3cFvgqK2z7yxF-n-r6p6MqLa1_XD8K7B4FIOtrI2JRM1rdB33Q_QZhTo4MCNHV_ninMz9Isxpq5bBXwKBSwQLa_SOVGVNa-K6afWyUVhNkYYgrF2ogaGKt3Io7DAmHRNc58wDUvfCRss4Q"
