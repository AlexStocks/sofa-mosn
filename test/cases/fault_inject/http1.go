package main

import (
	"fmt"
	"net/http"
	"time"

	"sofastack.io/sofa-mosn/test/lib"
	testlib_http "sofastack.io/sofa-mosn/test/lib/http"
)

// a config template for fault inject
const ConfigStrTmpl = `{
	"servers":[
		{
			"default_log_path":"stdout",
			"default_log_level": "DEBUG",
			"listeners":[
				{
					"address":"127.0.0.1:2045",
					"bind_port": true,
					"log_path": "stdout",
					"log_level": "FATAL",
					"filter_chains": [{
						 "filters": [
						 	{
								"type": "proxy",
								"config": {
									"downstream_protocol": "Http1",
									"upstream_protocol": "Http1",
									"router_config_name":"route"
								}
							},
							{
								"type": "connection_manager",
								"config": {
									"router_config_name":"route",
									 "virtual_hosts":[{
										 "name":"hosts",
										 "domains": ["*"],
										 "routers": [
										 	{
												"match":{
													"prefix":"/"
												},
												"route":{"cluster_name":"server_cluster"}
											}
										 ]
									 }]
								}
							}
						 ]
					}],
					"stream_filters": [
						{
							"type": "fault",
							"config": {
								"delay": {
									"fixed_delay": "1s",
									"percentage": %d
								},
								"abort": {
									"status": 500,
									"percentage": %d
								},
								"upstream_cluster": "",
								"headers": [
									{
										"name": "fault_inject",
										"value": "true"
									}
								]
							}
						}
					]
				}
			]
		}
	],
	"cluster_manager":{
		"clusters":[
			{
				"name": "server_cluster",
				"type": "SIMPLE",
				"lb_type": "LB_RANDOM",
				"hosts":[
					{"address":"127.0.0.1:8080"}
				]
			}
		]
	}
}`

func main() {
	testCases := []func() bool{
		InjectDelay,
		InjectAbort,
		InjectBoth,
	}
	for _, tc := range testCases {
		lib.Execute(tc)
		time.Sleep(time.Second)
	}
}

// case 1 :inject delay when request matched. inject delay 1s
// verify:
// 1. if send a request matched inject condition, expect the min rt > inject rt
// 2. if send a request not matched inject condition, expect the max rt < inject rt
// TODO: mosn response flag is inject if matched
func InjectDelay() bool {
	fmt.Println("----- Run boltv1 inject delay test ")
	// inject 100% delay, 0% abort (no abort)
	configStr := fmt.Sprintf(ConfigStrTmpl, 100, 0)
	mosn := lib.StartMosn(configStr)
	defer mosn.Stop()

	srv := testlib_http.NewMockServer("127.0.0.1:8080", nil)
	go srv.Start()
	defer srv.Close()

	// wait server start
	time.Sleep(time.Second)

	// client config
	// send a request matched the inject
	cltVerify := &testlib_http.VerifyConfig{
		ExpectedStatus: http.StatusOK,
		MinRT:          time.Second,
	}
	cltConfig := &testlib_http.ClientConfig{
		Addr:        "127.0.0.1:2045",
		MakeRequest: testlib_http.BuildHTTP1Request,
		RequestHeader: map[string]string{
			"fault_inject": "true",
		},
		Verify: cltVerify.Verify,
	}
	clt := testlib_http.NewClient(cltConfig, 1)
	if !clt.SyncCall() {
		fmt.Println("inject delay verify failed")
		return false
	}
	// change client config, send a request not matched the inject
	delete(cltConfig.RequestHeader, "fault_inject")
	// change the verify
	cltVerify.MinRT = 0

	if !clt.SyncCall() {
		fmt.Println("inject delay not matched verify failed")
		return false
	}
	// server will receive the requests
	if srv.ServerStats.RequestStats() != 2 {
		fmt.Println("server doest not receive enough requests, expected 2, but got ", srv.ServerStats.RequestStats())
		return false
	}
	// TODO: mosn access_log for response flag
	fmt.Println("----- PASS  boltv1 inject delay test ")
	return true
}

// case 2: inject abort when request matched. abort status code is 500(HTTP), for boltv1 is STATUS_UNKNOWN
// 1. if send a request matched, expect status is ERROR, and server will not receive a request
// 2. if send a request not matched, expect status is SUCCESS, and server will receive a request
func InjectAbort() bool {
	fmt.Println("----- Run boltv1 inject abort test ")
	// inject 0% delay(no delay), 100% abort
	configStr := fmt.Sprintf(ConfigStrTmpl, 0, 100)
	mosn := lib.StartMosn(configStr)
	defer mosn.Stop()

	srv := testlib_http.NewMockServer("127.0.0.1:8080", nil)
	go srv.Start()
	defer srv.Close()

	// wait server start
	time.Sleep(time.Second)

	// client config
	// send a request matched the inject
	cltVerify := &testlib_http.VerifyConfig{
		ExpectedStatus: http.StatusInternalServerError,
	}
	cltConfig := &testlib_http.ClientConfig{
		Addr:        "127.0.0.1:2045",
		MakeRequest: testlib_http.BuildHTTP1Request,
		RequestHeader: map[string]string{
			"fault_inject": "true",
		},
		Verify: cltVerify.Verify,
	}
	clt := testlib_http.NewClient(cltConfig, 1)
	if !clt.SyncCall() {
		fmt.Println("inject abort verify failed")
		return false
	}
	// Verify server will receive no requests
	if srv.ServerStats.RequestStats() != 0 {
		fmt.Println("server receive a request, but expected not")
		return false
	}

	// change the verify
	cltVerify.ExpectedStatus = http.StatusOK
	// change the request send
	delete(cltConfig.RequestHeader, "fault_inject")

	if !clt.SyncCall() {
		fmt.Println("inject abort not matched verify failed")
		return false
	}
	// server will receive the requests
	if srv.ServerStats.RequestStats() != 1 {
		fmt.Println("server doest not receive enough requests, expected 1, but got ", srv.ServerStats.RequestStats())
		return false
	}
	// TODO: mosn access_log for response flag

	fmt.Println("----- PASS  boltv1 inject abort test ")
	return true

}

// case 3: inject both delay and abort
// like case1 and case2
func InjectBoth() bool {
	fmt.Println("----- Run boltv1 inject delay and abort test ")
	configStr := fmt.Sprintf(ConfigStrTmpl, 100, 100)
	mosn := lib.StartMosn(configStr)
	defer mosn.Stop()

	srv := testlib_http.NewMockServer("127.0.0.1:8080", nil)
	go srv.Start()
	defer srv.Close()

	// wait server start
	time.Sleep(time.Second)

	// client config
	// send a request matched the inject
	cltVerify := &testlib_http.VerifyConfig{
		ExpectedStatus: http.StatusInternalServerError,
		MinRT:          time.Second,
	}
	cltConfig := &testlib_http.ClientConfig{
		Addr:        "127.0.0.1:2045",
		MakeRequest: testlib_http.BuildHTTP1Request,
		RequestHeader: map[string]string{
			"fault_inject": "true",
		},
		Verify: cltVerify.Verify,
	}
	clt := testlib_http.NewClient(cltConfig, 1)
	if !clt.SyncCall() {
		fmt.Println("inject delay and abort verify failed")
		return false
	}
	// Verify server will receive no requests
	if srv.ServerStats.RequestStats() != 0 {
		fmt.Println("server receive a request, but expected not")
		return false
	}
	// change client config, send a request not matched the inject
	delete(cltConfig.RequestHeader, "fault_inject")
	// change the client verify
	cltVerify.ExpectedStatus = http.StatusOK
	cltVerify.MinRT = 0
	if !clt.SyncCall() {
		fmt.Println("inject delay and abort not matched verify failed")
		return false
	}
	// server will receive the requests
	if srv.ServerStats.RequestStats() != 1 {
		fmt.Println("server doest not receive enough requests, expected 1, but got ", srv.ServerStats.RequestStats())
		return false
	}
	// TODO: mosn access_log for response flag
	fmt.Println("----- PASS  boltv1 inject delay and abort test ")
	return true

}