package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
)

const SERVER_ADDR = "http://127.0.0.1:8080/"

func StartServer(cb func(*http.Client)) {
	server_chan := make(chan bool, 1)
	server_process_chan := make(chan *exec.Cmd, 1)

	go func() {
		cmd := exec.Command("go", "run", "main.go")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
			Setpgid: true,
		}

		err := cmd.Start()
		if err != nil {
			server_chan <- false
			return
		}

		time.Sleep(3 * time.Second)
		server_process_chan <- cmd
		server_chan <- true
		cmd.Wait()
	}()

	defer func() {
		cmd := <- server_process_chan
		if cmd != nil && cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Process.Kill()
		}
	}()

	if <-server_chan {
		cookieJar, _ := cookiejar.New(nil)
		cli := &http.Client{
			Timeout: time.Second * 1,
			Jar:     cookieJar,
		}
		cb(cli)
	}
}

func TestShortening(t *testing.T) {
	StartServer(func(cli *http.Client) {
		true_url := "https://www.reddit.com"
		resp := apitest.New().
				EnableNetworking(cli).
				Post(SERVER_ADDR).
				FormData("url", true_url).
				Expect(t).
				Assert(jsonpath.Chain().Equal("true_url", true_url).Present("short_url").End()).
				Status(http.StatusOK).
				End().Response
		
		decoder := json.NewDecoder(resp.Body)
		var data struct {
			Short_url string `json:"short_url"`
			True_url string `json:"true_url"`
		}
		  
		err := decoder.Decode(&data)
		if err != nil {
			log.Fatal(err)
		}

		apitest.New().
				EnableNetworking(cli).
				Get(SERVER_ADDR + data.Short_url).
				Expect(t).
				Assert(jsonpath.Chain().Equal("url", data.True_url).End()).
				Status(http.StatusOK).
				End()
	})
}

func TestShorteningBadURL(t *testing.T) {
	StartServer(func(cli *http.Client) {
		true_url := "htttp://ww.google.c"
		apitest.New().
				EnableNetworking(cli).
				Post(SERVER_ADDR).
				FormData("url", true_url).
				Expect(t).
				Assert(jsonpath.Chain().Present("error").End()).
				Status(http.StatusOK).
				End()
	})
}