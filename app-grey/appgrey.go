package appgrey

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// 接口灰度条件类型
type apiGreyConditionType string

const (
	// 判断值是否在 Args 中
	in apiGreyConditionType = "in"
	// 判断值是否正则匹配 Args 的任何一个表达式
	pattern apiGreyConditionType = "pattern"
)

// 接口灰度条件
type apiGreyCondition struct {
	Type apiGreyConditionType `json:"type"`
	Args []string             `json:"args"`
}

func (this apiGreyCondition) match(value string) bool {
	if this.Args == nil {
		return false
	}
	// 根据类型处理条件判断
	switch this.Type {
	case in:
		// TODO: 待优化为 Set 结构缓存起来, 大数据量判断效率更高, 但似乎 go 没有原生 set 数据类型, 考虑 map 手动实现但需处理序列化问题
		for _, item := range this.Args {
			if item == value {
				return true
			}
		}
		break
	case pattern:
		for _, pattern := range this.Args {
			if isMatch, _ := regexp.MatchString(pattern, value); isMatch {
				return true
			}
		}
		break
	}
	return false
}

type apiGreyRule map[ApiGreyDimension]apiGreyCondition

func (this apiGreyRule) match(dimension map[ApiGreyDimension]string) (isMatch bool) {
	if dimension == nil {
		return false
	}
	// 判断规则中的每个条件是否都匹配
	for greyDimension, condition := range this {
		// 取出条件对应的值
		value := dimension[greyDimension]
		if value == "" {
			return false
		}
		// 判断条件是否匹配
		if !condition.match(value) {
			return false
		}
	}
	return true
}

// 接口灰度策略
type apiGreyStrategy struct {
	Name string          `json:"name,omitempty"`
	//Description string   `json:"description,omitempty"`
	Host string          `json:"host,omitempty"`
	Port int             `json:"port,omitempty"`
	Enable bool          `json:"enable,omitempty"`
	// 每个灰度策略 对应多个灰度规则
	// 每个灰度规则 由多个灰度条件构成
	Rules []apiGreyRule  `json:"rules"`
}

// 应用灰度策略
type appGreyStrategy struct {
	Version int64                        `json:"version"`
	// 对灰度策略解析无关的变量 直接忽略，减少内存占用
	//UpdatedBy string                     `json:"updatedBy"`
	//UpdatedAt time.Time                  `json:"updatedAt"`
	Host string                              `json:"host"`
	Port int                                 `json:"port"`
	Enable bool                              `json:"enable,omitempty"`
	Apis map[ApiGreyIdentify]apiGreyStrategy `json:"apis"`
}

// 根据灰度策略判断当前流量是否为灰度流量
func (this *appGreyStrategy) match(identify *ApiGreyIdentify, dimension map[ApiGreyDimension]string) (isGreyFlow bool, host string, port int) {
	// 当判断过程中发生 panic 时判断为 非灰度流量, 对外屏蔽异常
	defer func() {
		if p := recover(); p != nil {
			isGreyFlow = false
			// TODO: 待改为使用日志组件记录 panic 信息
			fmt.Printf("AppGrey match strategy panic： %v\n", p)
		}
	}()

	if this.Enable && this.Apis != nil && this.Host != "" && this.Port != 0 {
		if api, ok := this.Apis[*identify]; ok {
			host = this.Host
			port = this.Port
			if api.Host != "" {
				host = api.Host
			}
			if api.Port != 0 {
				port = api.Port
			}
			if api.Enable && api.Rules != nil {
				// 策略中任意一个规则匹配 则 返回匹配成功
				for _, rule := range api.Rules {
					// 处理每个规则的匹配
					if rule.match(dimension) {
						return true, host, port
					}
				}
			}
		}
	}

	return false, "", 0
}

type Repository interface {
	GetAppGreyStrategy() (string, error)
}

type appGrey struct {
	strategy   *appGreyStrategy
	repository Repository
	ticker     *time.Ticker
}

func (this *appGrey) loadStrategy() error {
	// 查询灰度放量策略的 JSON 字符串
	document, err := this.repository.GetAppGreyStrategy()
	if err != nil {
		return err
	}

	strategy := &appGreyStrategy{}
	// 转换为 appGreyStrategy 结构体
	err = json.Unmarshal([]byte(document), strategy)
	if err != nil {
		return err
	}

	if this.strategy == nil || this.strategy.Version < strategy.Version {
		this.strategy = strategy
		// TODO: 待改为使用日志组件打印正常日志
		//fmt.Printf("AppGrey load strategy from json： %v\n", document)
	} else {
		// TODO: 待改为使用日志组件打印正常日志
		fmt.Printf("AppGrey load strategy pass version： %v\n", strategy.Version)
	}
	return nil
}

func (this *appGrey) Initialize(repository Repository, updateInterval time.Duration) error {
	this.repository = repository
	if err := this.loadStrategy(); err != nil {
		return err
	}
	// 处理定时更新规则, 只允许执行一次
	if this.ticker == nil {
		this.ticker = time.NewTicker(updateInterval)
		// 周期性的 每 updateInterval 尝试获取灰度策略更新至内存
		go func() {
			for range this.ticker.C {
				if err := this.loadStrategy(); err != nil {
					// TODO: 待改为使用日志组件打印异常日志
					fmt.Printf("AppGrey ticker load strategy failed： %v\n", err)
				}
			}
		}()
	}
	return nil
}

// 判断一个请求是否为灰度请求
// 参数: identify 建议配置在常量中, dimension 根据产品要求添加,
// 返回: isGreyFlow 判断结果, true 是灰度请求, 后续应进行请求转发. false 表明不是灰度请求, 正常处理即可
// host 和 port 只有在 isGreyFlow 为 true 时才有意义, 表明请求需要转发到的灰度应用的 host 和 port.
func (this *appGrey) Match(identify ApiGreyIdentify, dimension map[ApiGreyDimension]string) (isGreyFlow bool, host string, port int) {
	if this.strategy != nil {
		return this.strategy.match(&identify, dimension)
	}
	// 在灰度放量规则未成功加载至内存期间，直接返回不匹配
	return false, "", 0
}

// 全局变量 对外统一暴露该实例
var AppGrey = &appGrey{}