package api

type HTTPErrorResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type HTTPMessageResp struct {
	Message string `json:"message"`
}
