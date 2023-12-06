package goshorty

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
	"sync"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/sqids/sqids-go"
	"gopkg.in/yaml.v3"
)

// Default APIConfig file path.
const API_CONFIG_FILE_PATH = "config/api_config.yaml"

// ShortyApp represents the application itself.
// To create an instance, first create an APIConfig, then use NewShortyApp().
type ShortyApp struct {
	Config APIConfig;
	// DB to store URL -> short ID maps in.
	DB *sql.DB;
	// counter is one of the inputs for short ID generation.
	counter uint64;
	mu sync.Mutex;
}

// APIConfig represents the API's actual configuration.
// To actually create one, use CreateAppConfig()
type APIConfig struct {
	// A Postgres connection string.
	Conninfo string `yaml:"conninfo"`;
	// Number of API requests per minute
	RequestsPerMinute float64 `yaml:"requestsPerMinute"`;
	// Address to listen for HTTP requests on, defaults to 0.0.0.0:8080
	ListenAddr string `yaml:"listenAddr"`;
}

// isUrlOk validates a given URL in a few ways.
// It can fail if the URL isn't reachable with a HEAD request.
func isUrlOk(u string) bool {
	if len(u) == 0 {
		return false
	}

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

	_, err = http_client.Do(req)
	if err != nil {
		log.Println(err)
		return false
	}

	return govalidator.IsURL(u)
}

// SetupRouter initializes a gin.Engine and sets the appropriate handlers.
// The handlers here are the meat of the application, responsible for shortening URLs and redirection.
func (shorty *ShortyApp) SetupRouter() *gin.Engine {
	s, err := sqids.New(sqids.Options{
		MinLength: 15,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	//https://github.com/gin-gonic/gin/issues/2809
	err = r.SetTrustedProxies(nil)
	if err != nil {
		log.Fatal("failed to set trusted proxies")
	}

	limiter := tollbooth.NewLimiter(shorty.Config.RequestsPerMinute, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Minute})
	limiter.SetMethods([]string{"POST"})
	limiter.SetMessage(`{"error": "too many requests"}`)
	limiter.SetMessageContentType("application/json; charset=utf-8")

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/shorten", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "https://github.com/eu90h/goshorty")
	})

	r.POST("/shorten", tollbooth_gin.LimitHandler(limiter), func(c *gin.Context) {
		if shorty.DB == nil {
			db, err := sql.Open("postgres", shorty.Config.Conninfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			shorty.DB = db
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
		short_url, err := s.Encode([]uint64{shorty.counter, x.Uint64()})
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusOK, gin.H{"error": "failed to shorten url"})
			return
		}
		_, err = shorty.DB.Query(`INSERT INTO urlmap (short_url,true_url,creation_time,clicks) VALUES ($1,$2,$3,0)`, short_url, true_url, time.Now())
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusOK, gin.H{"error": "failed to shorten url"})
			return
		}
		shorty.mu.Lock()
		shorty.counter += 1
		shorty.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"short_url": short_url, "true_url": true_url})
	})

	r.GET("/", func (c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "https://github.com/eu90h/goshorty")
	})

	r.GET("/:id", func(c *gin.Context) {
		if shorty.DB == nil {
			db, err := sql.Open("postgres", shorty.Config.Conninfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			shorty.DB = db
		}
		var true_url string
		shortened_url_id := c.Params.ByName("id")
		if err := shorty.DB.QueryRow(`SELECT (true_url) from urlmap where short_url = $1`, shortened_url_id).Scan(&true_url); err != nil {
			c.JSON(http.StatusOK, gin.H{"error": "shortened url not found"})
			return
		}
		shorty.DB.QueryRow(`UPDATE urlmap SET clicks = clicks + 1 WHERE short_url = $1`, shortened_url_id)
		c.Redirect(http.StatusMovedPermanently, true_url)
	})

	return r
}

// CreateAppConfig creates a new APIConfig by filling in the fields from a number of sources.
// Environment variables are checked first, then `config/api_config.yaml` is checked.
func CreateAppConfig() APIConfig {
	api_config := APIConfig{}
	if len(os.Getenv("DATABASE_URL")) > 0 {
		api_config.Conninfo = os.Getenv("DATABASE_URL")
	}

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

	_, err := os.Stat(API_CONFIG_FILE_PATH)
	if err == nil {
		content, err := os.ReadFile(API_CONFIG_FILE_PATH)
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

func NewShortyApp(config *APIConfig) *ShortyApp {
	var api_config = config
	if config == nil {
		x := CreateAppConfig()
		api_config = &x
	}
	shorty := ShortyApp{}
	shorty.Config = *api_config
	return &shorty
}