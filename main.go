package main

import (
	"crypto/rand"
	"database/sql"
	"log"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/sqids/sqids-go"
	"gopkg.in/yaml.v3"
)

type APIConfig struct {
	Conninfo string `yaml:"conninfo"`;
	RequestsPerMinute float64 `yaml:"requestsPerMinute"`;
	ListenAddr string `yaml:"listenAddr"`;
}

var db *sql.DB

func isUrlOk(u string) bool {
	http_client := http.Client{
		Timeout: 3 * time.Second,
	}

	_, err := url.ParseRequestURI(u)
	if err != nil {
		log.Println(err)
		return false
	}

	req, err := http.NewRequest("HEAD", u, nil)
	if err != nil {
		log.Println(err)
		return false
	}

	resp, err := http_client.Do(req)
	if err != nil {
		log.Println(err)
		return false
	}

	if resp.StatusCode != 200 {
		log.Println(err)
		return false
	}

	return true
}

func setupRouter(api_config *APIConfig) *gin.Engine {
	var counter uint64 = 0;
	if api_config == nil {
		log.Fatal("api_config is nil")
	}

	s, err := sqids.New(sqids.Options{
		MinLength: 15,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	
	limiter := tollbooth.NewLimiter(api_config.RequestsPerMinute, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Minute})
	limiter.SetMethods([]string{"POST"})
	limiter.SetMessage(`{"error": "too many requests"}`)
	limiter.SetMessageContentType("application/json; charset=utf-8")

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.POST("/", tollbooth_gin.LimitHandler(limiter), func(c *gin.Context) {
		if db == nil {
			db, err = sql.Open("postgres", api_config.Conninfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}
		true_url := c.Request.FormValue("url")
		if !isUrlOk(true_url) {
			c.JSON(http.StatusOK, gin.H{"error": "invalid url"})
			return
		}
		x, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusOK, gin.H{"error": "failed to shorten url"})
		}
		short_url, err := s.Encode([]uint64{counter, x.Uint64()})
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusOK, gin.H{"error": "failed to shorten url"})
			return
		}
		_, err = db.Query(`INSERT INTO urlmap (short_url,true_url) VALUES ($1,$2)`, short_url, true_url)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusOK, gin.H{"error": "failed to shorten url"})
			return
		}

		counter += 1
		c.JSON(http.StatusOK, gin.H{"short_url": short_url, "true_url": true_url})
	})

	r.GET("/:id", func(c *gin.Context) {
		if db == nil {
			db, err = sql.Open("postgres", api_config.Conninfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}
		var true_url string
		shortened_url_id := c.Params.ByName("id")
		if err := db.QueryRow(`SELECT (true_url) from urlmap where short_url = $1`, shortened_url_id).Scan(&true_url); err != nil {
			c.JSON(http.StatusOK, gin.H{"error": "shortened url not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": true_url})
	})

	return r
}

func CreateAppConfig() APIConfig {
	api_config := APIConfig{}
	if len(os.Getenv("GOSHORTY_CONNINFO")) > 0 {
		api_config.Conninfo = os.Getenv("GOSHORTY_CONNINFO")
	}

	api_config.ListenAddr = "0.0.0.0:8080"
	if len(os.Getenv("GOSHORTY_LISTENADDR")) > 0 {
		api_config.ListenAddr = os.Getenv("GOSHORTY_LISTENADDR")
	}

	api_config.RequestsPerMinute = 60.0
	if len(os.Getenv("GOSHORTY_REQUESTSPERMINUTE")) > 0 {
		x, err := strconv.ParseFloat(os.Getenv("GOSHORTY_REQUESTSPERMINUTE"), 64)
		if err != nil {
			log.Println(err)
		} else {
			api_config.RequestsPerMinute = x
		}
	}

	_, err := os.Stat("api_config.yaml")
	if err == nil {
		content, err := os.ReadFile("api_config.yaml")
		if err != nil {
			log.Fatal(err)
		}

		err = yaml.Unmarshal([]byte(content), &api_config)
		if err != nil {
			log.Fatal(err)
		}
	}

	return api_config
}

func main() {
	api_config := CreateAppConfig()
	log.Println(api_config)
	r := setupRouter(&api_config)
	err := r.Run(api_config.ListenAddr) // TODO: change to RunTLS for HTTPS support.
	if err != nil {
		log.Fatal(err)
	}
}
