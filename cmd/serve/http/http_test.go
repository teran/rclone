package http

import (
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	_ "github.com/ncw/rclone/local"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("updategolden", false, "update golden files for regression test")

const (
	testBindAddress = "localhost:51777"
	testURL         = "http://" + testBindAddress + "/"
)

func startServer(t *testing.T, f fs.Fs) {
	s := newServer(f, testBindAddress)
	go s.serve()

	// try to connect to the test server
	pause := time.Millisecond
	for i := 0; i < 10; i++ {
		conn, err := net.Dial("tcp", testBindAddress)
		if err == nil {
			_ = conn.Close()
			return
		}
		// t.Logf("couldn't connect, sleeping for %v: %v", pause, err)
		time.Sleep(pause)
		pause *= 2
	}
	t.Fatal("couldn't connect to server")

}

func TestInit(t *testing.T) {
	// Configure the remote
	fs.LoadConfig()
	// fs.Config.LogLevel = fs.LogLevelDebug
	// fs.Config.DumpHeaders = true
	// fs.Config.DumpBodies = true

	// exclude files called hidden.txt and directories called hidden
	require.NoError(t, fs.Config.Filter.AddRule("- hidden.txt"))
	require.NoError(t, fs.Config.Filter.AddRule("- hidden/**"))

	// Create a test Fs
	f, err := fs.NewFs("testdata/files")
	require.NoError(t, err)

	startServer(t, f)
}

// check body against the file, or re-write body if -updategolden is
// set.
func checkGolden(t *testing.T, fileName string, got []byte) {
	if *updateGolden {
		t.Logf("Updating golden file %q", fileName)
		err := ioutil.WriteFile(fileName, got, 0666)
		require.NoError(t, err)
	} else {
		want, err := ioutil.ReadFile(fileName)
		require.NoError(t, err)
		wants := strings.Split(string(want), "\n")
		gots := strings.Split(string(got), "\n")
		assert.Equal(t, wants, gots, fileName)
	}
}

func TestGET(t *testing.T) {
	for _, test := range []struct {
		URL    string
		Status int
		Golden string
		Method string
		Range  string
	}{
		{
			URL:    "",
			Status: http.StatusOK,
			Golden: "testdata/golden/index.html",
		},
		{
			URL:    "notfound",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/notfound.html",
		},
		{
			URL:    "dirnotfound/",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/dirnotfound.html",
		},
		{
			URL:    "hidden/",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/hiddendir.html",
		},
		{
			URL:    "one%25.txt",
			Status: http.StatusOK,
			Golden: "testdata/golden/one.txt",
		},
		{
			URL:    "hidden.txt",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/hidden.txt",
		},
		{
			URL:    "three/",
			Status: http.StatusOK,
			Golden: "testdata/golden/three.html",
		},
		{
			URL:    "three/a.txt",
			Status: http.StatusOK,
			Golden: "testdata/golden/a.txt",
		},
		{
			URL:    "",
			Method: "HEAD",
			Status: http.StatusOK,
			Golden: "testdata/golden/indexhead.txt",
		},
		{
			URL:    "one%25.txt",
			Method: "HEAD",
			Status: http.StatusOK,
			Golden: "testdata/golden/onehead.txt",
		},
		{
			URL:    "",
			Method: "POST",
			Status: http.StatusMethodNotAllowed,
			Golden: "testdata/golden/indexpost.txt",
		},
		{
			URL:    "one%25.txt",
			Method: "POST",
			Status: http.StatusMethodNotAllowed,
			Golden: "testdata/golden/onepost.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusOK,
			Golden: "testdata/golden/two.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusPartialContent,
			Range:  "bytes=2-5",
			Golden: "testdata/golden/two2-5.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusPartialContent,
			Range:  "bytes=0-6",
			Golden: "testdata/golden/two-6.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusPartialContent,
			Range:  "bytes=3-",
			Golden: "testdata/golden/two3-.txt",
		},
	} {
		method := test.Method
		if method == "" {
			method = "GET"
		}
		req, err := http.NewRequest(method, testURL+test.URL, nil)
		require.NoError(t, err)
		if test.Range != "" {
			req.Header.Add("Range", test.Range)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, test.Status, resp.StatusCode, test.Golden)
		body, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)

		checkGolden(t, test.Golden, body)
	}
}
