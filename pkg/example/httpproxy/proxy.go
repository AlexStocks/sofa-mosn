package main

import (
	"fmt"
	"gitlab.alipay-inc.com/afe/mosn/pkg/types"
	"gitlab.alipay-inc.com/afe/mosn/pkg/api/v2"
	"net/http"
	"gitlab.alipay-inc.com/afe/mosn/pkg/log"
	_ "gitlab.alipay-inc.com/afe/mosn/pkg/router/basic"
	"time"
	"net"
	"gitlab.alipay-inc.com/afe/mosn/pkg/upstream/cluster"
	"gitlab.alipay-inc.com/afe/mosn/pkg/server"
	"gitlab.alipay-inc.com/afe/mosn/pkg/server/config/proxy"
	"io/ioutil"
	"gitlab.alipay-inc.com/afe/mosn/pkg/protocol"
)

const (
	RealServerAddr  = "127.0.0.1:8088"
	RealServerAddr2 = "127.0.0.1:8089"
	MeshServerAddr  = "127.0.0.1:2044"
	TestCluster1     = "tstCluster1"
	TestCluster2     = "tstCluster2"
	TestListenerRPC = "tstListener"
)

func main() {
	go func() {
		// pprof server
		http.ListenAndServe("0.0.0.0:9090", nil)
	}()

	log.InitDefaultLogger("", log.DEBUG)

	stopChan := make(chan bool)
	meshReadyChan := make(chan bool)

	go func() {
		// upstream1
		server := &http.Server{
			Addr:         RealServerAddr,
			Handler:      &serverHandler{"ups1"},
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		server.ListenAndServe()
	}()

	go func() {
		// upstream2
		server := &http.Server{
			Addr:         RealServerAddr2,
			Handler:      &serverHandler{"ups2"},
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		server.ListenAndServe()
	}()

	select {
	case <-time.After(2 * time.Second):
	}

	go func() {
		//  mesh
		cmf := &clusterManagerFilterRPC{}
		cm := cluster.NewClusterManager(nil, nil, nil, false)

		//RPC
		srv := server.NewServer(&server.Config{}, cmf, cm)

		srv.AddListener(rpcProxyListener(), &proxy.GenericProxyFilterConfigFactory{
			Proxy: genericProxyConfig(),
		}, nil)
		cmf.cccb.UpdateClusterConfig(clustersrpc())
		cmf.chcb.UpdateClusterHost(TestCluster1, 0, rpchosts1())
		cmf.chcb.UpdateClusterHost(TestCluster2, 0, rpchosts2())

		meshReadyChan <- true

		srv.Start() //开启连接

		select {
		case <-stopChan:
			srv.Close()
		}
	}()

	go func() {
		select {
		case <-meshReadyChan:
			// client
			tr := &http.Transport{
			}

			httpClient := http.Client{Transport: tr}
			req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/hahaha.htm?key1=valuex&nobody=true", MeshServerAddr), nil)
			req.Header.Add("service", "com.alipay.rpc.common.service.facade.SampleService:1.0")
			resp, err := httpClient.Do(req)

			if err != nil {
				fmt.Printf("[CLIENT]receive err %s", err)
				fmt.Println()
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)

			if err != nil {
				fmt.Printf("[CLIENT]receive err %s", err)
				fmt.Println()
				return
			}

			fmt.Printf("[CLIENT]receive data %s", body)
			fmt.Println()
		}
	}()

	select {
	case <-time.After(time.Second * 10):
		stopChan <- true
		fmt.Println("[MAIN]closing..")
	}
}

type serverHandler struct {
	tag string
}

func (sh *serverHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	sh.ShowRequestInfoHandler(w, req)
}

func (sh *serverHandler) ShowRequestInfoHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[UPSTREAM %s]receive request %s", sh.tag, r.URL)
	fmt.Println()

	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintf(w, "Method: %s\n", r.Method)
	fmt.Fprintf(w, "Protocol: %s\n", r.Proto)
	fmt.Fprintf(w, "Host: %s\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr: %s\n", r.RemoteAddr)
	fmt.Fprintf(w, "RequestURI: %q\n", r.RequestURI)
	fmt.Fprintf(w, "URL: %#v\n", r.URL)
	fmt.Fprintf(w, "Body.ContentLength: %d (-1 means unknown)\n", r.ContentLength)
	fmt.Fprintf(w, "Close: %v (relevant for HTTP/1 only)\n", r.Close)
	fmt.Fprintf(w, "TLS: %#v\n", r.TLS)
	fmt.Fprintf(w, "\nHeaders:\n")

	r.Header.Write(w)
}

func genericProxyConfig() *v2.Proxy {
	proxyConfig := &v2.Proxy{
		DownstreamProtocol: string(protocol.Http1),
		UpstreamProtocol:   string(protocol.Http1),
	}

	header1 := v2.HeaderMatcher{
		Name:  "service",
		Value: "com.alipay.rpc.common.service.facade.SampleService:1.0",
	}

	header2 := v2.HeaderMatcher{
		Name:  "service",
		Value: "tst",
	}

	router1V2 := v2.Router{
		Match: v2.RouterMatch{
			Headers: []v2.HeaderMatcher{header1},
		},

		Route: v2.RouteAction{
			ClusterName: TestCluster1,
		},
	}

	router2V2 := v2.Router{
		Match: v2.RouterMatch{
			Headers: []v2.HeaderMatcher{header2},
		},

		Route: v2.RouteAction{
			ClusterName: TestCluster2,
		},
	}

	router3V2 := v2.Router{
		Match: v2.RouterMatch{
			Headers: []v2.HeaderMatcher{header1},
			Path: "/hahaha.htm",
		},

		Route: v2.RouteAction{
			ClusterName: TestCluster2,
		},
	}

	proxyConfig.VirtualHosts = append(proxyConfig.VirtualHosts, &v2.VirtualHost{
		Name:    "testSofaRoute",
		Domains: []string{"*"},
		Routers: []v2.Router{router1V2, router2V2, router3V2},
	})
	return proxyConfig
}

func rpcProxyListener() *v2.ListenerConfig {
	addr, _ := net.ResolveTCPAddr("tcp", MeshServerAddr)

	return &v2.ListenerConfig{
		Name:                    TestListenerRPC,
		Addr:                    addr,
		BindToPort:              true,
		PerConnBufferLimitBytes: 1024 * 32,
		LogPath:                 "",
		LogLevel:                uint8(log.DEBUG),
		DisableConnIo:           true,
	}
}

func rpchosts1() []v2.Host {
	var hosts []v2.Host

	hosts = append(hosts, v2.Host{
		Address: RealServerAddr,
		Weight:  100,
	})

	return hosts
}

func rpchosts2() []v2.Host {
	var hosts []v2.Host

	hosts = append(hosts, v2.Host{
		Address: RealServerAddr2,
		Weight:  100,
	})

	return hosts
}

type clusterManagerFilterRPC struct {
	cccb types.ClusterConfigFactoryCb
	chcb types.ClusterHostFactoryCb
}

func (cmf *clusterManagerFilterRPC) OnCreated(cccb types.ClusterConfigFactoryCb, chcb types.ClusterHostFactoryCb) {
	cmf.cccb = cccb
	cmf.chcb = chcb
}

func clustersrpc() []v2.Cluster {
	var configs []v2.Cluster
	configs = append(configs, v2.Cluster{
		Name:              TestCluster1,
		ClusterType:       v2.SIMPLE_CLUSTER,
		LbType:            v2.LB_RANDOM,
		MaxRequestPerConn: 1024,
	})

	configs = append(configs, v2.Cluster{
		Name:              TestCluster2,
		ClusterType:       v2.SIMPLE_CLUSTER,
		LbType:            v2.LB_RANDOM,
		MaxRequestPerConn: 1024,
	})

	return configs
}

//type MyHandler struct {
//	foobar string
//}
//
//// request handler in net/http style, i.e. method bound to MyHandler struct.
//func (h *MyHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
//	// notice that we may access MyHandler properties here - see h.foobar.
//	fmt.Fprintf(ctx, "Hello, world! Requested path is %q. Foobar is %q",
//		ctx.Path(), h.foobar)
//}
//
//// request handler in fasthttp style, i.e. just plain function.
//func fastHTTPHandler(ctx *fasthttp.RequestCtx) {
//	fmt.Fprintf(ctx, "Hi there! RequestURI is %q", ctx.RequestURI())
//}
//
//func main(){
//
//	fmt.Println("hello fasthttp")
//
//	// pass bound struct method to fasthttp
//	myHandler := &MyHandler{
//		foobar: "foobar",
//	}
//	fasthttp.ListenAndServe(":8080", myHandler.HandleFastHTTP)
//
//	fmt.Println("hello fasthttp")
//
//	// pass plain function to fasthttp
//	fasthttp.ListenAndServe(":8081", fastHTTPHandler)
//
//
//	wg := sync.WaitGroup{}
//	wg.Add(1)
//
//	fmt.Println("hello fasthttp")
//
//	wg.Wait()
//}
