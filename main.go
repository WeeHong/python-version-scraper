package main

import (
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

var regex = regexp.MustCompile(`\d+(\.\d+)+`)

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

	router.HandleFunc("/python-prerelease", func(rw http.ResponseWriter, r *http.Request) {
		res, err := getPrereleaseVersion()
		if err != nil {
			log.Fatal(err)
		}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(res))
	}).Methods("GET")

	http.ListenAndServe(":"+os.Getenv("PORT"), httpRateLimiter.RateLimit(router))
}

func getStableVersion() (string, error) {
	v, err := version.NewVersion("0")
	if err != nil {
		return "", fmt.Errorf("latest version: %s", err.Error())
	}

	c := colly.NewCollector()

	c.OnHTML(".col-row.two-col .column:first-child a[href]", func(e *colly.HTMLElement) {
		v, err = extractVersion(v, e.Attr("href"))
		if err != nil {
			log.Fatal(err)
		}
	})

	c.Visit("https://www.python.org/downloads/source/")

	return v.String(), nil
}

func getPrereleaseVersion() (string, error) {
	v, err := version.NewVersion("0")
	if err != nil {
		return "", fmt.Errorf("prerelease version: %s", err.Error())
	}

	c := colly.NewCollector()

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		v, err = extractVersion(v, e.Attr("href"))
		if err != nil {
			log.Fatal(err)
		}
	})

	c.Visit("https://www.python.org/ftp/python/")

	return v.String(), nil
}

func extractVersion(v *version.Version, cur string) (*version.Version, error) {
	matched := regex.FindAllString(cur, -1)

	if len(matched) > 0 {
		current, err := version.NewVersion(matched[0])
		if err != nil {
			return nil, fmt.Errorf("extract version: %s", err.Error())
		}
		if v.LessThan(current) {
			v = current
		}
	}

	return v, nil
}
