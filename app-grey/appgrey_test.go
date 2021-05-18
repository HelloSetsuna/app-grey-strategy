package appgrey

import (
	"fmt"
	"testing"
	"time"
)

// 模拟实现 Repository 接口, 实际使用时需根据具体场景实现该接口，如从数据库查询或接口查询得到
type AppGreyRepositoryExample struct {}
func (*AppGreyRepositoryExample) GetAppGreyStrategy() (string, error)  {
	var json = `{
		"version": 0, 
		"updatedBy": "某某某", 
		"updatedAt": "2021-05-13 15:52:00", 
		"host": "grey.app-server.com", 
		"port": 8080,
		"enable": true,
		"apis": {
			"online-json-direct": {
				"enable": true, 
				"host": "grey.app-server.com", 
				"port": 8081,
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
				"enable": false,
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
	return json, nil
}

// 使用样例
func ExampleMatch() {
	// 初始化 只需在应用启动时调用一次, 传入 自定义的配置获取接口 实现，指定多久刷新一次配置
	if err := AppGrey.Initialize(&AppGreyRepositoryExample{}, time.Second * 5); err != nil {
		fmt.Printf("AppGrey initialize failed: %v\n", err)
	}


	// 开始进行 某类业务接口的灰度维度数据匹配，判断是否为一个灰度流量
	isGreyFlow, host, port := AppGrey.Match(OnlineJsonDirect, map[ApiGreyDimension]string{
		// 在处理某类业务接口请求时，解析相关的维度数据, 对于 OnlineJsonDirect 的两个灰度规则, 命中第一个
		Version: "v1",
		StoreId: "SID000002",
		InsCode: "INS0001xx",
	})

	if isGreyFlow {
		// TODO: 此时应进行灰度流量的转发操作 并中止后续的正常处理
		fmt.Printf("isGreyFlow: %v, host: %v, port: %v \n", isGreyFlow, host, port)
		return
	}

	// 继续后续的业务处理
	fmt.Printf("isGreyFlow: false\n")

	// Output:
	// isGreyFlow: true, host: grey.app-server.com, port: 8081
}

// 单元测试
func TestMatch(t *testing.T) {
	if err := AppGrey.Initialize(&AppGreyRepositoryExample{}, time.Second * 5); err != nil {
		fmt.Printf("AppGrey initialize failed: %v\n", err)
	}

	var tests = []struct{
		identify ApiGreyIdentify
		dimension map[ApiGreyDimension]string
		isGreyFlow bool
		host string
		port int
	}{
		{OnlineJsonDirect, map[ApiGreyDimension]string{
			Version: "v1",
			StoreId: "SID000002",
			InsCode: "",
		}, true, "grey.app-server.com", 8081 },

		{OnlineJsonDirect, map[ApiGreyDimension]string{
			Version: "v2",
			StoreId: "SID000002",
			InsCode: "",
		}, false, "", 0 },

		{OnlineJsonDirect, map[ApiGreyDimension]string{
			Version: "v2",
			StoreId: "SID000002",
			InsCode: "INS0002xx",
		}, true, "grey.app-server.com", 8081 },

		{OnlineJsonDirect, map[ApiGreyDimension]string{
			Version: "v2",
			StoreId: "SID000002",
			InsCode: "INS0001xx",
		}, false, "", 0 },
	}

	for _, test := range tests {
		isGreyFlow, host, port := AppGrey.Match(test.identify, test.dimension)
		if isGreyFlow != test.isGreyFlow || host != test.host || port != test.port {
			t.Errorf("AppGrey.Match(%v,%v) = (%v,%v,%v), want (%v,%v,%v)", test.identify, test.dimension,
				isGreyFlow, host, port, test.isGreyFlow, test.host, test.port)
		}
	}
}
