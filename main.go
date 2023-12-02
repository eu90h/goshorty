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
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/sqids/sqids-go"
	"gopkg.in/yaml.v3"
)

type APIConfig struct {
	Conninfo string
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

func setupRouter() *gin.Engine {
	var counter uint64 = 0;

	if db == nil {
		log.Fatal("no db connection")
	}

	s, err := sqids.New(sqids.Options{
		MinLength: 15,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.POST("/", func(c *gin.Context) {
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

func main() {
	content, err := os.ReadFile("api_config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	api_config := APIConfig{}
    
	err = yaml.Unmarshal([]byte(content), &api_config)
	if err != nil {
		log.Fatal(err)
	}

	db, err = sql.Open("postgres", api_config.Conninfo)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	r := setupRouter()
	err = r.Run("127.0.0.1:8080") // TODO: change to RunTLS for HTTPS support.
	if err != nil {
		log.Fatal(err)
	}
}
