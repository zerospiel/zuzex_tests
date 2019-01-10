package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

const layout = "01/02/2006"

func copyParams(params url.Values) url.Values {
	p := url.Values{}
	for k, v := range params {
		p.Set(k, v[0])
	}
	return p
}

func getHeader() (header *http.Header, params url.Values, err error) {
	res, err := http.Get("https://miamidade.realtdm.com/public/cases/list")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	cookie := res.Header.Get("Set-Cookie")

	params = url.Values{}
	params.Set("filterCasesPerPage", "10")
	params.Set("filterFiltered", "1")
	params.Set("isPublic", "1")
	params.Set("filterSaleDateStop", time.Now().AddDate(0, 0, 30).Format(layout))
	params.Set("filterSaleDateStart", time.Now().AddDate(0, 0, -30).Format(layout))

	header = &http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Cookie":       []string{cookie},
	}

	return
}

func getSearchResults(page int, header *http.Header, params url.Values) (results string, err error) {
	params = copyParams(params)
	params.Set("filterPageNumber", fmt.Sprintf("%d", page))

	req, err := http.NewRequest(http.MethodPost, "https://miamidade.realtdm.com/public/cases/list", bytes.NewBufferString(params.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if res == nil {
		log.Fatal("res nil")
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		log.Fatal("400+ error")
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	results = string(b)

	return
}

func main() {
	h, p, err := getHeader()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(h, p)

	result, err := getSearchResults(2, h, p)
	if err != nil {
		log.Fatal(err)
	}

	ioutil.WriteFile("test.html", []byte(result), 0644)
}
