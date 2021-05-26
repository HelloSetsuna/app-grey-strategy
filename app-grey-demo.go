package main

import (
	"appgrey"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// 这里将灰度规则的初始值直接写死在全局变量中
var greyStrategyJson = `{
		"version": 0, 
		"updatedBy": "某某某", 
		"updatedAt": "2021-05-13 15:52:00", 
		"defaultHost": "localhost", 
		"defaultPort": 8083,
		"host": "localhost", 
		"port": 8081,
		"enable": true,
		"apis": {
			"online-json-direct": {
				"enable": true, 
				"defaultHost": "localhost", 
				"defaultPort": 8083,
				"host": "localhost", 
				"port": 8081,
				"name": "线下联机JSON直连业务", 
				"description": "基于 xxx 协议, 提供消费, 预授权, 退货...",
				"rules": [
					{
						"host": "localhost", 
						"port": 8081,
						"conditions": {
							"version": {
								"type": "in", 
								"args": ["v1"] 
							},
							"storeId": {
								"type": "in", 
								"args": ["SID000001", "SID000002"]
							}
						}
					},
					{
						"port": 8082,
						"conditions": {
							"insCode": {
								"type": "pattern", 
								"args": ["^INS0002.*", "^INS0003.*"]
							}
						}
					}
				]
			},
			"online-json-indirect": {
				"enable": true,
				"name": "线下联机JSON间连业务",
				"description": "基于 xxx 协议, 提供消费, 预授权, 退货...",
				"rules": [
					{
						"conditions": {
							"version": {
								"type": "in", 
								"args": ["v2", "v3"]
							},
							"insCode": {
								"type": "in", 
								"args": ["INS000001"]
							}
						}
					}
				]
			}
		}
	}`

// 模拟实现 Repository 接口, 实际使用时需根据具体场景实现该接口，如从数据库查询或接口查询得到
type AppGreyRepositoryExample struct {}
func (*AppGreyRepositoryExample) GetAppGreyStrategy() (string, error)  {
	return greyStrategyJson, nil
}

func gatewayBoot()  {
	if err := appgrey.AppGrey.Initialize(&AppGreyRepositoryExample{}, time.Second * 5); err != nil {
		fmt.Printf("AppGrey initialize failed: %v\n", err)
	}

	router := httprouter.New()
	// 查询灰度策略
	router.GET("/grey/strategy/json", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Write([]byte(greyStrategyJson))
	})
	// 更新灰度策略
	router.PUT("/grey/strategy/json", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if bytes, err := ioutil.ReadAll(r.Body); err == nil {
			greyStrategyJson = string(bytes)
			if err := appgrey.AppGrey.LoadStrategy(); err != nil {
				fmt.Printf("AppGrey LoadStrategy failed: %v\n", err)
			}
		}
	})

	router.GET("/api/:version/business/:identify", func (w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		params := struct {
			StoreId string
			InsCode string
			TerminalId string
			// ...
		}{}

		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err == nil {
			json.Unmarshal(bodyBytes, &params)
		}

		isGreyFlow, host, port := appgrey.AppGrey.Match(appgrey.ApiGreyIdentify(p.ByName("identify")),
			map[appgrey.ApiGreyDimension]string{
				// 在处理某类业务接口请求时，解析相关的维度数据, 对于 OnlineJsonDirect 的两个灰度规则, 命中第一个
				appgrey.Version: p.ByName("version"),
				appgrey.StoreId: params.StoreId,
				appgrey.InsCode: params.InsCode,
				appgrey.TerminalId: params.TerminalId,
			})
		fmt.Printf("params:%v, isGreyFlow:%v, host:%v, port:%v\n", params, isGreyFlow, host, port)

		// 代理请求
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes)) // 为了确保 r.Body 可以被再次读取
		if proxyUrl, err := url.Parse(fmt.Sprintf("http://%v:%d", host, port)); err == nil {
			httputil.NewSingleHostReverseProxy(proxyUrl).ServeHTTP(w, r)
		} else {
			fmt.Printf("[error] illegal url format http://%v:%d err: %v", host, port, err)
		}
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", 8080), router))
}

func serverBoot(port int)  {
	router := httprouter.New()
	router.GET("/api/:version/business/:identify", func (w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		params := struct {
			StoreId string
			InsCode string
			TerminalId string
			// ...
		}{}

		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err == nil {
			json.Unmarshal(bodyBytes, &params)
		}

		w.Write([]byte(fmt.Sprintf("response %v by server(port: %v)", r.RequestURI, port)))
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), router))
}

func main() {
	var port int
	var gateway bool
	var server bool
	flag.BoolVar(&gateway, "gateway",false,"是否为流量调度DEMO")
	flag.BoolVar(&server, "server",false,"是否为服务端DEMO")
	flag.IntVar( &port, "port",8080 ,"应用监听的端口")
	flag.Parse()
	fmt.Printf("args gateway:%v, server:%v, port:%v\n", gateway, server, port)
	if gateway {
		gatewayBoot()
	}
	if server {
		serverBoot(port)
	}

}