# 应用灰度放量策略解析模块

由于现存业务接口协议格式复杂，在需要按指定维度将部分业务接口的请求流量转至灰度环境处理时，在网络层面上无法进行分流。为解决该问题最终方案为由生产环境的应用内部根据灰度放量策略进行分流，将灰度请求重定向到灰度环境的应用。为此需设计灰度放量策略的配置格式，提供基础模块，便于开发人员使用此模块以快速的进行业务接口的灰度改造. 应用在 k8s 中某个环境的灰度环境和原环境以 namespace 区分即可.

## 配置格式
每个应用有多种类型的业务接口, 而每个业务接口可能有多个放量规则, 每个放量策略将由多个维度匹配规则, 据此设计如下格式:
```json
// 整个流量调度策略是一个JSON文档
{
    "version": 0, // 版本号用于标识文档版本
    "updatedBy": "某某某", // 更新人
    "updatedAt": "2021-05-13 15:52:00", // 更新时间
    "defaultHost": "server-host", // 默认路由的 灰度环境域名
    "defaultPort": 8080, // 默认路由的 灰度环境端口
    "host": "server-host-grey", // 默认转发的 灰度环境域名
    "port": 8080, // 默认转发的 灰度环境端口
    "apis": {
        // 某类业务接口 的 调度策略配置, ApiTransform 拿到调度策略配置后
        // 根据 online-json-direct (业务接口类型标识)找到该配置内容, 判断是否灰度
        "online-json-direct": {
            "enable": true, // 启用或禁用 该类业务接口
            "defaultHost": "server-host", // 默认路由的 灰度环境域名
            "defaultPort": 8080, // 默认路由的 灰度环境端口
            "host": "server-host-grey", // 可覆盖默认的 灰度环境域名
            "port": 8081, // 可覆盖默认的 灰度环境端口
            "name": "线下联机JSON直连业务", // UI 展示用途
            "description": "基于 xxx 协议, 提供外卡消费, 预授权, 退货...",
            // 每个业务接口 可以有多个调度规则
            "rules": [
                // 某个调度规则定义, 其由多个维度条件匹配构成
                {
                    "host": "server-host-grey", // 针对规则 可单独配置转发的域名
                    "port": 8082, // 针对规则 可单独配置转发的端口
                    "conditions": {
                        // 要求 版本号(version) 是 v1
                        "version": {
                            "type": "in", // 匹配类型 便于拓展
                            "args": ["v1"] // 匹配参数 必须是数组 内部元素字符串
                        },
                        // 并且 门店号(storeId) 是 SID000001 或 SID000002
                        "storeId": {
                            "type": "in", 
                            "args": ["SID000001", "SID000002"]
                        }
                    }
                },
                {
                    "conditions": {
                        "institutionCode": {
                            "type": "pattern", 
                            "args": ["^INS0002.*", "^INS0003.*"]
                        }
                    }
                }
            ]
        },
        // 另一类业务接口
        "/online-json-indirect/*": {
            "enable": false, // 启用或禁用 该类业务接口
            "name": "线下联机JSON间连业务",
            "description": "基于 xxx 协议, 提供外卡消费, 预授权, 退货...",
            "rules": [
                {
                    "conditions": {
                        "version": {
                            "type": "in", 
                            "args": ["v2", "v3"]
                        },
                        "institutionCode": {
                            "type": "in", 
                            "args": ["INS000001"]
                        }
                    }
                }
            ]
        }
    }
}
```
## 模块使用

### 1. 实现 appgrey.Repository 接口
由于不同应用的配置管理实现方式各不相同, 如存放数据库中或通过接口拉取, 或 Consul 等第三方配置中心等, 故在使用此模块前需 先实现模块的 appgrey.Repository 接口, 其定义如下:
```go
type Repository interface {
    // 查询 灰度放量策略, 返回其配置的 json 字符串 或 error
    GetAppGreyStrategy() (string, error)
}
```

### 2. 初始化 AppGrey 实例
为了便于开发人员使用, appgrey.AppGrey 本身是一个全局变量, 但在其默认为未初始化状态, 在未初始化时调用其 Match 方法将用于返回 false 表明该请求不是灰度流量, 只有在初始化后才会根据灰度放量配置正确判定请求是否为灰度流量. 在初始化完成后 appgrey.AppGrey 会自动启动一个 ticker 来按指定时间自动重新获取配置, 以达到动态刷新配置的目的

```go
func main() {
    // ...
	
    // 初始化 只需在应用启动时调用一次, 传入 自定义的配置获取接口 实现，指定多久自动重新获取一次配置
    if err := AppGrey.Initialize(&AppGreyRepositoryExample{}, time.Second * 5); err != nil {
        fmt.Printf("AppGrey initialize failed: %v\n", err) // 初始化异常应用自行处理
    }
    
    // ...
}
```

### 3. 使用 AppGrey 判定灰度流量

方法签名如下:
```go
// 判断一个请求是否为灰度请求
// 参数: identify 建议配置在常量中, dimension 根据产品要求添加灰度维度数据,
// 返回: isGreyFlow 判断结果, true 是灰度请求, 后续应进行请求转发. false 表明不是灰度请求, 正常处理即可
// host 和 port 只有在 isGreyFlow 为 true 时才有意义, 表明请求需要转发到的灰度应用的 host 和 port.
func (this *appGrey) Match(identify ApiGreyIdentify, dimension map[ApiGreyDimension]string) (isGreyFlow bool, host string, port int)
```

调用样例如下:
```go
// 开始进行 某类业务接口的 灰度维度数据 匹配，判断是否为一个灰度流量
isGreyFlow, host, port := AppGrey.Match(OnlineJsonDirect, map[ApiGreyDimension]string{
    // 在处理某类业务接口请求时，解析相关的维度数据, 对于 OnlineJsonDirect 的两个灰度规则, 命中第一个
    Version: "v1",
    StoreId: "SID000002",
    InsCode: "INS0001xx",
})
```

## 样例DEMO
查看 app-grey-demo.go 文件, 该 DEMO 模拟了一个简单的业务处理系统使用该 灰度放量策略解析模块 转发灰度流量的功能, 其内部写死了一套默认的JSON配置
### 1. 运行 app-grey-demo.exe 
```shell
# 在 windows 开启三个命令行分别执行如下命令
# 命令行一:
app-grey-demo
# 命令行二:
app-grey-demo -grey -port 8081
# 命令行三:
app-grey-demo -grey -port 8082
```
### 2. 导入 POSTMAN 的配置文件
打开 POSTMAN 将 app-grey-demo.postman_collection.json 文件导入, 具体参数看 POSTMAN 的配置, 可调整参数和更新灰度放量策略配置 测试
#### GET http://localhost:8080/grey/strategy/json
> 查看应用当前的灰度放量策略配置
#### PUT http://localhost:8080/grey/strategy/json
> 更新应用当前的灰度放量策略配置
#### GET http://localhost:8080/api/v1/business/online-json-direct
> 测试 online-json-direct 的业务接口灰度放量策略, 可以看到是有 8082 的 DEMO 返回的
#### GET http://localhost:8080/api/v1/business/online-json-indirect
> 测试 online-json-indirect 的业务接口灰度放量策略, 可以看到是有 8081 的 DEMO 返回的

## 实践后记
接触 golang 时间较短, 尝试编写了该模块后在有些地方理解更加清晰了, 简洁的 go test, 简洁的 包 及 封装. 待后续迁移至公司仓库.