package appgrey

// 业务接口类型标识
type ApiGreyIdentify string

const (
	OnlineJsonDirect   ApiGreyIdentify = "online-json-direct"
	OnlineJsonIndriect ApiGreyIdentify = "online-json-indirect"



	// TODO: 对于后续业务方需要添加的新的灰度接口类型，应追加在此注释上方
)