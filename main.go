package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/gocolly/colly/v2"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-version"
	"github.com/joho/godotenv"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	store, err := memstore.New(65536)
	if err != nil {
		log.Fatal(err)
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerMin(20),
		MaxBurst: 5,
	}

	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		log.Fatal(err)
	}

	httpRateLimiter := throttled.HTTPRateLimiter{
		RateLimiter: rateLimiter,
		VaryBy:      &throttled.VaryBy{Path: true},
	}

	router := mux.NewRouter()

	router.HandleFunc("/python-stable", func(rw http.ResponseWriter, r *http.Request) {
		res, err := getStableVersion()
		if err != nil {
			log.Fatal(err)
		}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(res))
	}).Methods("GET")

	router.HandleFunc("/python-prereleases", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("Under Implementation"))
	}).Methods("GET")

	fmt.Println(os.Getenv("PORT"))

	http.ListenAndServe(":"+os.Getenv("PORT"), httpRateLimiter.RateLimit(router))
}

func getStableVersion() (string, error) {
	v, err := version.NewVersion("0")
	if err != nil {
		return "", errors.New("Latest version: " + err.Error())
	}

	c := colly.NewCollector()

	c.OnHTML(".col-row.two-col .column:first-child a[href]", func(e *colly.HTMLElement) {
		v = versionChecker(v, e.Attr("href"))
	})

	// c.Visit("https://www.python.org/ftp/python/")
	c.Visit("https://www.python.org/downloads/source/")

	return v.String(), nil
}

func versionChecker(v *version.Version, cur string) *version.Version {
	regex := regexp.MustCompile(`\d+(\.\d+)+`)
	matched := regex.FindAllString(cur, -1)

	if len(matched) > 0 {
		fmt.Println(matched[0])

		current, err := version.NewVersion(matched[0])
		if err != nil {
			panic(err.Error())
		}
		if v.LessThan(current) {
			v = current
		}
	}

	return v
}
