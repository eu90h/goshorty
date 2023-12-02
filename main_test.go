package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	"gopkg.in/yaml.v3"
)

const SERVER_ADDR = "http://127.0.0.1:8080/"
var rate_limit int

func TestMain(m *testing.M) {
	content, err := os.ReadFile("api_config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	api_config := APIConfig{}
	err = yaml.Unmarshal([]byte(content), &api_config)
	if err != nil {
		log.Fatal(err)
	}

	rate_limit = int(api_config.RequestsPerMinute)
	code := m.Run()
    os.Exit(code)
}

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

func TestShorteningRateLimiter(t *testing.T) {
	log.Printf("rate limit is %d\n", rate_limit)
	StartServer(func(cli *http.Client) {
		true_url := "https://www.reddit.com"
		for i := 0; i < 5+rate_limit; i++ {
			req, err := http.NewRequest("POST", SERVER_ADDR, bytes.NewBuffer([]byte("url="+true_url)))
			if err != nil {
				log.Println(err)
			}

			c := http.Client{Timeout: time.Duration(1) * time.Second}
			_, err = c.Do(req)
			if err != nil {
				log.Println(err)
			}
		}
		apitest.New().
				EnableNetworking(cli).
				Post(SERVER_ADDR).
				FormData("url", true_url).
				Expect(t).
				Assert(jsonpath.Chain().Present("error").End()).
				Status(http.StatusTooManyRequests).
				End()
	})
}