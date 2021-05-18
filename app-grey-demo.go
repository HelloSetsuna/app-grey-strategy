package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/HelloSetsuna/app-grey-strategy"
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
		"host": "localhost", 
		"port": 8081,
		"enable": true,
		"apis": {
			"online-json-direct": {
				"enable": true, 
				"host": "localhost", 
				"port": 8082,
				"name": "线下联机JSON直连业务", 
				"description": "基于 xxx 协议, 提供消费, 预授权, 退货...",
				"rules": [
					{
						"version": {
							"type": "in", 
							"args": ["v1"] 
						},
						"storeId": {
							"type": "in", 
							"args": ["SID000001", "SID000002"]
						}
					},
					{
						"insCode": {
							"type": "pattern", 
							"args": ["^INS0002.*", "^INS0003.*"]
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
						"version": {
							"type": "in", 
							"args": ["v2", "v3"]
						},
						"insCode": {
							"type": "in", 
							"args": ["INS000001"]
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

func main() {
	var port int
	var grey bool
	flag.IntVar( &port, "port",8080 ,"应用监听的端口")
	flag.BoolVar(&grey, "grey",false,"是否为灰度应用") // 实际环境应该使用 环境变量来区分
	flag.Parse()
	fmt.Printf("args grey:%v, port:%v", grey, port)

	if !grey { // 非灰度环境才拉取初始化配置
		if err := appgrey.AppGrey.Initialize(&AppGreyRepositoryExample{}, time.Second * 5); err != nil {
			fmt.Printf("AppGrey initialize failed: %v\n", err)
		}
	}

	router := httprouter.New()
	router.GET("/grey/strategy/json", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Write([]byte(greyStrategyJson))
	})
	router.PUT("/grey/strategy/json", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if bytes, err := ioutil.ReadAll(r.Body); err == nil {
			greyStrategyJson = string(bytes)
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

		if !grey {
			isGreyFlow, host, port := appgrey.AppGrey.Match(appgrey.ApiGreyIdentify(p.ByName("identify")),
				map[appgrey.ApiGreyDimension]string{
					// 在处理某类业务接口请求时，解析相关的维度数据, 对于 OnlineJsonDirect 的两个灰度规则, 命中第一个
					appgrey.Version: p.ByName("version"),
					appgrey.StoreId: params.StoreId,
					appgrey.InsCode: params.InsCode,
					appgrey.TerminalId: params.TerminalId,
				})
			fmt.Printf("params:%v, isGreyFlow:%v, host:%v, port:%v\n", params, isGreyFlow, host, port)
			if isGreyFlow {
				// 代理请求
				r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes)) // 为了确保 r.Body 可以被再次读取
				if proxyUrl, err := url.Parse(fmt.Sprintf("http://%v:%d", host, port)); err == nil {
					httputil.NewSingleHostReverseProxy(proxyUrl).ServeHTTP(w, r)
					return
				}
			}
		}

		w.Write([]byte(fmt.Sprintf("response by server(grey:%v,port:%v)", grey, port)))
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), router))
}