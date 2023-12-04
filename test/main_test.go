package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"testing"
	"time"

	goshorty "github.com/eu90h/goshorty/pkg"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
)

var server_addr *url.URL
var rate_limit int = 60

func TestMain(m *testing.M) {
	api_config := goshorty.CreateAppConfig()
	rate_limit = int(api_config.RequestsPerMinute)
	if len(os.Getenv("GOSHORTY_TEST_SERVER_ADDR")) > 0 {
		u, err :=  url.Parse(os.Getenv("GOSHORTY_TEST_SERVER_ADDR"))
		if err != nil {
			panic(err)
		}
		server_addr = u
	}
	if server_addr == nil {
		u, err := url.Parse("http://127.0.0.1:8080")
		if err != nil {
			panic(err)
		}
		if u == nil {
			panic("server address is nil")
		}
		server_addr = u
	}
	StartServer()
	time.Sleep(5 * time.Second)
	code := m.Run()
    os.Exit(code)
}

func StartServer() {
	go func() {
		shorty := goshorty.NewShortyApp(nil)
		r := shorty.SetupRouter()
		if r == nil {
			log.Fatal("router was nil")
		}
		err := r.Run(shorty.Config.ListenAddr) // TODO: change to RunTLS for HTTPS support.
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func TestShortening(t *testing.T) {
	cookieJar, _ := cookiejar.New(nil)
	cli := &http.Client{
		Timeout: time.Second * 10,
		Jar:     cookieJar,
	}
	log.Println(server_addr.JoinPath("shorten").String())
	true_url := "https://www.reddit.com"
	resp := apitest.New().
			EnableNetworking(cli).
			Post(server_addr.JoinPath("shorten").String()).
			FormData("url", true_url).
			Expect(t).
			Assert(jsonpath.Chain().Equal("true_url", true_url).Present("short_url").End()).
			Status(http.StatusOK).
			End().Response

	decoder := json.NewDecoder(resp.Body)
	var data struct {
		ShortUrl string `json:"short_url"`
		TrueUrl string `json:"true_url"`
	}

	err := decoder.Decode(&data)
	if err != nil {
		log.Fatal(err)
	}
}

func TestShorteningBadURL(t *testing.T) {
	cookieJar, _ := cookiejar.New(nil)
	cli := &http.Client{
		Timeout: time.Second * 10,
		Jar:     cookieJar,
	}
	true_url := "htttp://ww.google.c"
	apitest.New().
			EnableNetworking(cli).
			Post(server_addr.JoinPath("shorten").String()).
			FormData("url", true_url).
			Expect(t).
			Assert(jsonpath.Chain().Present("error").End()).
			Status(http.StatusOK).
			End()
}

func TestShorteningRateLimiter(t *testing.T) {
	cookieJar, _ := cookiejar.New(nil)
	cli := &http.Client{
		Timeout: time.Second * 10,
		Jar:     cookieJar,
	}
	true_url := "https://www.reddit.com"
	for i := 0; i < 5+rate_limit; i++ {
		req, err := http.NewRequest("POST", server_addr.JoinPath("shorten").String(), bytes.NewBuffer([]byte("url="+true_url)))
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
			Post(server_addr.JoinPath("shorten").String()).
			FormData("url", true_url).
			Expect(t).
			Assert(jsonpath.Chain().Present("error").End()).
			Status(http.StatusTooManyRequests).
			End()
}

func TestPing(t *testing.T) {
	cookieJar, _ := cookiejar.New(nil)
	cli := &http.Client{
		Timeout: time.Second * 10,
		Jar:     cookieJar,
	}
	apitest.New().
			EnableNetworking(cli).
			Get(server_addr.JoinPath("/ping").String()).
			Expect(t).
			Body("pong").
			Status(http.StatusOK).
			End()
}