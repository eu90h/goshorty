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

func TestShortening(t *testing.T) {
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
		true_url := "https://www.reddit.com"
		handler := func(w http.ResponseWriter, r *http.Request) {
			
		}

		cookieJar, _ := cookiejar.New(nil)
		cli := &http.Client{
			Timeout: time.Second * 1,
			Jar:     cookieJar,
		}
		
		resp := apitest.New().
				HandlerFunc(handler).
				EnableNetworking(cli).
				Post("http://127.0.0.1:8080/").
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
				HandlerFunc(handler).
				EnableNetworking(cli).
				Get("http://127.0.0.1:8080/" + data.Short_url).
				Expect(t).
				Assert(jsonpath.Chain().Equal("url", data.True_url).End()).
				Status(http.StatusOK).
				End()
	}
}