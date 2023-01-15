package jtrpc

import (
	"encoding/json"
	"github.com/avast/retry-go"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"strings"
	"time"
)

type Response struct {
	NetId   int
	Message string
	Result  *gjson.Result
}

type Request struct {
	Uuid       string // 本次请求的id
	HttpMethod string
	Url        string
	ParamBody  string
	RouteName  string
	Token      string
	NetName    string
	NetId      int
	HeaderMap  map[string]string
}

type RPCJitu struct {
}

var jtClient = &http.Client{}

func (rPcJitu *RPCJitu) Request(request Request, reply *Response) error {
	start := time.Now()
	log.Debugf("------>RPC调用来了%v", request.Uuid)
	var result *gjson.Result
	if request.HttpMethod == "GET" {
		result = RequestJson("GET", request.Url, nil, request.RouteName, request.Token, request.HeaderMap)
	} else if request.HttpMethod == "POST" {
		var data = strings.NewReader(request.ParamBody)
		result = RequestJson("POST", request.Url, data, request.RouteName, request.Token, request.HeaderMap)
	} else {
		marshal, err := json.Marshal(reply)
		if err != nil {
			log.Error(err)
		}
		log.Errorf("请求错误 %v", marshal)
		jsonString := `{"msg":"方法不正确，当前是[` + request.HttpMethod + `]","code":405}`
		result1 := gjson.Parse(jsonString)
		result = &result1
	}
	reply.Result = result
	status := "未知"
	if reply.Result != nil {
		code := reply.Result.Get("code")
		if code.Exists() {
			status = code.Raw
		}

	}
	total := time.Now().Sub(start)
	log.Debugf("<------RPC返回了 %v,请求状态 %v  %v", request.Uuid, status, total)
	return nil
}

func RequestJson(method string, url string, paramBody io.Reader, routeName string, token string, headerMap map[string]string) *gjson.Result {
	var gjsonData *gjson.Result

	// 不采用401和403，因为402 不需要重试3次，直接放弃
	const jsonString = `{"msg":"token没有","code":402}`

	if strings.Contains(url, "jtexpress.com.cn") {
		//log.Infof("token信息 %s 参数 %v", token, paramBody)
		if len(token) < 10 {
			log.Infof("net信息2 %v", token)
			parse := gjson.Parse(jsonString)
			return &parse
		}
	} else {
		//	类似钉钉通知就不需要检查token了
	}

	retry.DefaultDelay = 2 * time.Second // 初次时间
	retry.DefaultAttempts = 1            // 最大尝试次数
	err := retry.Do(
		func() error {
			req, err := http.NewRequest(method, url, paramBody)
			if err != nil {
				log.Error(err)
				return err
			}
			var header = http.Header{}

			header.Set("authority", "jmsgw.jtexpress.com.cn")
			header.Set("accept", "application/json, text/plain, */*")
			header.Set("accept-language", "en-US,en;q=0.9")
			header.Set("cache-control", "max-age=2, must-revalidate")
			header.Set("content-type", "application/json;charset=UTF-8")
			header.Set("lang", "zh_CN")
			header.Set("origin", "https://jms.jtexpress.com.cn")
			header.Set("referer", "https://jms.jtexpress.com.cn/")
			header.Set("sec-ch-ua", `" Not A;Brand";v="99", "Chromium";v="101", "Microsoft Edge";v="101"`)
			header.Set("sec-ch-ua-mobile", "?0")
			header.Set("sec-ch-ua-platform", `"Windows"`)
			header.Set("sec-fetch-dest", "empty")
			header.Set("sec-fetch-mode", "cors")
			header.Set("sec-fetch-site", "same-site")
			header.Set("user-agent", "Mozilla/5.0 (X11; Windows x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.64 Safari/537.36 Edg/101.0.1210.47")

			req.Header = header
			req.Header.Set("routename", routeName)
			req.Header.Set("authtoken", token)
			if len(headerMap) == 0 {
				req.Header.Set("content-type", "application/json;charset=UTF-8")
			} else {
				for headerKey, headerValue := range headerMap {
					req.Header.Set(headerKey, headerValue)
				}
			}
			var resp *http.Response
			resp, err = jtClient.Do(req)
			if err != nil {
				log.Error(err)
				return err
			}
			defer resp.Body.Close()
			bodyText, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}

			parse := gjson.Parse(string(bodyText))
			gjsonData = &parse
			if err != nil {
				log.Fatal("something wrong when call NewFromReader")
			}
			return nil
		},
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			log.Infof("%s请求重试=%d", method, n)
			return retry.BackOffDelay(n, err, config)
		}),
	)
	if err != nil {
		log.Errorf("重试失败%s", url)
	}
	return gjsonData
}
