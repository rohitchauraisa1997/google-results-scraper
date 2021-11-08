package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
	domained "github.com/rohitchauraisa1997/google-results-scraper/domains"
)

type FormInput struct {
	searchTerm   string
	countryCode  string
	languageCode string
	pages        string
	count        string
	backoff      string
}

type Input struct {
	SearchTerm   string `json:"search_term"`
	CountryCode  string `json:"country_code"`
	LanguageCode string `json:"language_code"`
}

type SearchResult struct {
	ResultRank  int
	ResultURL   string
	ResultTitle string
	ResultDesc  string
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
	"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Safari/604.1.38",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:56.0) Gecko/20100101 Firefox/56.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Safari/604.1.38",
}

func randomUserAgent() string {
	rand.Seed(time.Now().Unix())
	randNum := rand.Int() % len(userAgents)
	return userAgents[randNum]
}

func buildGoogleUrls(searchTerm, countryCode, languageCode string, pages, count int) ([]string, error) {
	toScrape := []string{}
	searchTerm = strings.Trim(searchTerm, " ")
	searchTerm = strings.Replace(searchTerm, " ", "+", -1)
	if googleBase, found := domained.GoogleDomains[countryCode]; found {
		for i := 0; i < pages; i++ {
			start := i * count
			scrapeURL := fmt.Sprintf("%s%s&num=%d&hl=%s&start=%d&filter=0", googleBase, searchTerm, count, languageCode, start)
			toScrape = append(toScrape, scrapeURL)
		}
	} else {
		err := fmt.Errorf("country (%s) is currently not supported", countryCode)
		return nil, err
	}
	fmt.Println(strings.Repeat(".", 30))
	fmt.Println(toScrape)
	fmt.Println(strings.Repeat(".", 30))
	return toScrape, nil

}

func googleResultParsing(response *http.Response, rank int) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromResponse(response)

	if err != nil {
		return nil, err
	}

	results := []SearchResult{}
	sel := doc.Find("div.g")
	rank++
	for i := range sel.Nodes {
		item := sel.Eq(i)
		linkTag := item.Find("a")
		link, _ := linkTag.Attr("href")
		titleTag := item.Find("h3.r")
		descTag := item.Find("span.st")
		desc := descTag.Text()
		title := titleTag.Text()
		link = strings.Trim(link, " ")

		if link != "" && link != "#" && !strings.HasPrefix(link, "/") {
			result := SearchResult{
				rank,
				link,
				title,
				desc,
			}
			results = append(results, result)
			rank++
		}
	}
	return results, err

}

func getScrapeClient(proxyString interface{}) *http.Client {

	switch v := proxyString.(type) {

	case string:
		proxyUrl, _ := url.Parse(v)
		return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
	default:
		return &http.Client{}
	}
}

func GoogleScrape(searchTerm, countryCode, languageCode string, proxyString interface{}, pages, count, backoff int) ([]SearchResult, error) {
	results := []SearchResult{}
	resultCounter := 0
	googlePages, err := buildGoogleUrls(searchTerm, countryCode, languageCode, pages, count)
	if err != nil {
		return nil, err
	}
	for _, page := range googlePages {
		res, err := scrapeClientRequest(page, proxyString)
		if err != nil {
			return nil, err
		}
		data, err := googleResultParsing(res, resultCounter)
		if err != nil {
			return nil, err
		}
		resultCounter += len(data)
		for _, result := range data {
			results = append(results, result)
		}
		time.Sleep(time.Duration(backoff) * time.Second)
	}
	return results, nil
}

func scrapeClientRequest(searchURL string, proxyString interface{}) (*http.Response, error) {
	baseClient := getScrapeClient(proxyString)
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", randomUserAgent())

	res, err := baseClient.Do(req)
	if res.StatusCode != 200 {
		err := fmt.Errorf("scraper received a non-200 status code suggesting a ban")
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	return res, nil
}

func jsonResponse(w http.ResponseWriter, r *http.Request) {
	var input Input
	// _ = json.NewDecoder(r.Body).Decode(&input)
	_ = json.NewDecoder(r.Body).Decode(&input)
	searchTerm := input.SearchTerm
	countryCode := input.CountryCode
	languageCode := input.LanguageCode
	log.Printf("Getting info for searchTerm = %s countryCode = %s languageCode = %s", searchTerm, countryCode, languageCode)
	// res, err := GoogleScrape("ronaldo", "in", "en", nil, 1, 30, 10)
	res, err := GoogleScrape(searchTerm, countryCode, languageCode, nil, 1, 10, 10)
	if err == nil {
		for _, res := range res {
			fmt.Println(res)
		}
	}
	// res := make(map[string]interface{})
	json.NewEncoder(w).Encode(res)

}

func formResponse(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("input.html"))
	if r.Method != http.MethodPost {
		tmpl.Execute(w, nil)
		fmt.Println("returning")
		return
	}

	details := FormInput{
		searchTerm:   r.FormValue("Search Term"),
		countryCode:  r.FormValue("Country Code"),
		languageCode: r.FormValue("Language Code"),
		pages:        r.FormValue("Pages"),
		count:        r.FormValue("Count"),
		backoff:      r.FormValue("Backoff"),
	}
	fmt.Println(details)
	fmt.Println(details.searchTerm)
	fmt.Println(details.countryCode)
	fmt.Println(details.languageCode)

	pages, _ := strconv.Atoi(details.pages)
	count, _ := strconv.Atoi(details.count)
	backoff, _ := strconv.Atoi(details.backoff)

	res, err := GoogleScrape(details.searchTerm, details.countryCode, details.languageCode, nil, pages, count, backoff)
	if err == nil {
		for _, res := range res {
			fmt.Println(res)
		}
	}
	responseTmpl := template.Must(template.ParseFiles("response.html"))
	if r.Method != http.MethodPost {
		responseTmpl.Execute(w, nil)
		fmt.Println("returning")
		return
	}
	responseTmpl.Execute(w, res)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", jsonResponse).Methods("POST")
	// r.HandleFunc("/form", formResponse).Methods("POST")
	http.HandleFunc("/form", formResponse)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
