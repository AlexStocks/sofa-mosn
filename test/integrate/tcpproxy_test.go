package integrate

import (
	"testing"
	"time"

	"sofastack.io/sofa-mosn/pkg/mosn"
	"sofastack.io/sofa-mosn/pkg/protocol"
	"sofastack.io/sofa-mosn/pkg/types"
	testutil "sofastack.io/sofa-mosn/test/util"
)

type tcpExtendCase struct {
	*TestCase
}

func (c *tcpExtendCase) Start(isRouteEntryMode bool) {
	c.AppServer.GoServe()
	appAddr := c.AppServer.Addr()
	meshAddr := testutil.CurrentMeshAddr()
	c.ClientMeshAddr = meshAddr
	cfg := testutil.CreateTCPProxyConfig(meshAddr, []string{appAddr}, isRouteEntryMode)
	mesh := mosn.NewMosn(cfg)
	go mesh.Start()
	go func() {
		<-c.Finish
		c.AppServer.Close()
		mesh.Close()
		c.Finish <- true
	}()
	time.Sleep(5 * time.Second) //wait server and mesh start
}

const _NIL types.Protocol = "null"

func TestTCPProxy(t *testing.T) {
	appaddr := "127.0.0.1:8080"
	testCases := []*tcpExtendCase{
		&tcpExtendCase{NewTestCase(t, protocol.HTTP1, _NIL, testutil.NewHTTPServer(t, nil))},
		&tcpExtendCase{NewTestCase(t, protocol.HTTP2, _NIL, testutil.NewUpstreamHTTP2(t, appaddr, nil))},
		&tcpExtendCase{NewTestCase(t, protocol.SofaRPC, _NIL, testutil.NewRPCServer(t, appaddr, testutil.Bolt1))},
	}
	for i, tc := range testCases {
		t.Logf("start case #%d\n", i)
		tc.Start(false)
		go tc.RunCase(1, 0)
		select {
		case err := <-tc.C:
			if err != nil {
				t.Errorf("[ERROR MESSAGE] #%d tcp proxy test failed, protocol: %s, error: %v\n", i, tc.AppProtocol, err)
			}
		case <-time.After(15 * time.Second):
			t.Errorf("[ERROR MESSAGE] #%d tcp proxy hang, protocol: %s\n", i, tc.AppProtocol)
		}
		tc.FinishCase()
	}
}
func TestTCPProxyRouteEntry(t *testing.T) {
	appaddr := "127.0.0.1:8080"
	testCases := []*tcpExtendCase{
		&tcpExtendCase{NewTestCase(t, protocol.HTTP1, _NIL, testutil.NewHTTPServer(t, nil))},
		&tcpExtendCase{NewTestCase(t, protocol.HTTP2, _NIL, testutil.NewUpstreamHTTP2(t, appaddr, nil))},
		&tcpExtendCase{NewTestCase(t, protocol.SofaRPC, _NIL, testutil.NewRPCServer(t, appaddr, testutil.Bolt1))},
	}
	for i, tc := range testCases {
		t.Logf("start case #%d\n", i)
		tc.Start(true)
		go tc.RunCase(1, 0)
		select {
		case err := <-tc.C:
			if err != nil {
				t.Errorf("[ERROR MESSAGE] #%d tcp proxy route entry test failed, protocol: %s, error: %v\n", i, tc.AppProtocol, err)
			}
		case <-time.After(15 * time.Second):
			t.Errorf("[ERROR MESSAGE] #%d tcp proxy route entry hang, protocol: %s\n", i, tc.AppProtocol)
		}
		tc.FinishCase()
	}
}
