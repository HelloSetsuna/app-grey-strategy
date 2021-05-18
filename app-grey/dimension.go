package appgrey

// 业务接口灰度维度
type ApiGreyDimension string

const (
	Version    ApiGreyDimension = "version"
	StoreId    ApiGreyDimension = "storeId"
	InsCode    ApiGreyDimension = "insCode"
	TerminalId ApiGreyDimension = "terminalId"




	// TODO: 对于后续业务方需要添加的新的灰度维度，应追加在此注释上方
)