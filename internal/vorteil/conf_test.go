package vorteil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func handler(w http.ResponseWriter, r *http.Request) {

	fmt.Printf("URL %s\n", r.URL.Path)

	switch r.URL.Path {
	case "/computeMetadata/v1/instance/attributes/vorteil":
		{
			w.Write([]byte("vorteil"))
		}
	case "/computeMetadata/v1/instance/hostname":
		{
			w.Write([]byte("hostname"))
		}
	default:
		w.WriteHeader(404)
	}
}
func startHttpServer() *http.Server {

	http.HandleFunc("/", handler)

	server := &http.Server{Addr: "0.0.0.0:7777"}

	// go func() {
	// 	if err := server.ListenAndServe(); err != nil {
	// 	}
	// }()

	return server

}

func TestVCFG(t *testing.T) {
	// to test this function, run build.sh in test/demoapp
	if _, err := os.Stat("/tmp/demoapp.raw"); os.IsNotExist(err) {
		t.Fatalf("raw disk for test not found. run build.sh in test/demoapp")
	}

	v := New(LogFnStdout)

	v.readVCFG("/tmp/demoapp.raw")

}

func TestCloudConfig(t *testing.T) {

	v := New(LogFnStdout)

	srv := startHttpServer()

	time.Sleep(1 * time.Second)

	r := gcpReq
	r.server = "http://127.0.0.1:7777"
	probe(r, v)

	if err := srv.Shutdown(context.TODO()); err != nil {
		panic(err) // failure/timeout shutting down the server gracefully
	}

	// hv, c := hypervisorGuess(v, "unknown")
	// assert.Equal(t, hv, HV_UNKNOWN)
	// assert.Equal(t, c, CP_UNKNOWN)
	//
	// hv, c = hypervisorGuess(v, "SeaBIOS blah blah")
	// assert.Equal(t, hv, HV_KVM)
	// assert.Equal(t, c, CP_NONE)
	//
	// hv, c = hypervisorGuess(v, "innotek GmbH blah blah")
	// assert.Equal(t, hv, HV_VIRTUALBOX)
	// assert.Equal(t, c, CP_NONE)
	//
	// hv, c = hypervisorGuess(v, "Phoenix Technologies LTD blah blah")
	//
	// assert.Equal(t, hv, HV_VMWARE)
	// assert.Equal(t, c, CP_NONE)
	//
	// hv, c = hypervisorGuess(v, "Google blah blah")
	// assert.Equal(t, hv, HV_KVM)
	// assert.Equal(t, c, CP_GCP)
	//
	// hv, c = hypervisorGuess(v, "Amazon blah blah")
	// assert.Equal(t, hv, HV_KVM)
	// assert.Equal(t, c, CP_EC2)
	//
	// hv, c = hypervisorGuess(v, "Xen blah blah")
	// assert.Equal(t, hv, HV_XEN)
	// assert.Equal(t, c, CP_NONE)
	//
	// hv, c = hypervisorGuess(v, "American Megatrends Inc. blah blah")
	// assert.Equal(t, hv, HV_HYPERV)
	// assert.Equal(t, c, CP_NONE)

}
