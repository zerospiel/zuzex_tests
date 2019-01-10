package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	searchSite           = "https://miamidade.realtdm.com/public/cases/list"
	entryActiveStatus    = "ACTIVE"
	noCases              = "NO CASES FOUND!"
	noFilter             = "NO CASE FILTERS SELECTED!"
	layout               = "01/02/2006"
	saleDateTitle        = "Sale Date"
	propertyAddressTitle = "Property Address"

	resultsPerPage        = 100
	entriesToFetchPerOnce = 20

	// SELECTORS
	pageSelector       = "div.table-div.box-shadow.row-spacer > div.pagination-bar > div.text-right > div.pull-left.muted"
	noResultsSelector  = "div.table-div.box-shadow.row-spacer > div.padding10.text-center > div.text-huge.padding10"
	tableSelector      = "div.table-div.box-shadow.row-spacer > table#county-setup.table"
	rowOfTableSelector = "tr.load-case.table-row.link"
)

var (
	rxPageCount = regexp.MustCompile(`(?i)(?m)^Page\s([0-9]+\/([0-9]+))$`)
)

func copyParams(params url.Values) url.Values {
	p := url.Values{}
	for k, v := range params {
		p.Set(k, v[0])
	}
	return p
}

func getHeader() (header *http.Header, params url.Values, err error) {
	res, err := http.Get(searchSite)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	cookie := res.Header.Get("Set-Cookie")

	params = url.Values{}
	params.Set("filterCasesPerPage", strconv.Itoa(resultsPerPage))
	params.Set("filterFiltered", "1")
	params.Set("isPublic", "1")
	params.Set("filterSaleDateStop", time.Now().AddDate(0, 0, 31).Format(layout))
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

	req, err := http.NewRequest(http.MethodPost, searchSite, bytes.NewBufferString(params.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header = *header

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

func getActiveIDs(body string) (activeIDs []string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return
	}

	doc.Find(tableSelector).Each(func(i int, entryHtml *goquery.Selection) {
		entryHtml.Find(rowOfTableSelector).Each(func(i int, subEntryHtml *goquery.Selection) {
			status := subEntryHtml.Find("td.text-left").Text()
			if status == entryActiveStatus {
				id, ok := subEntryHtml.Attr("data-caseid")
				if !ok {
					return
				}
				activeIDs = append(activeIDs, id)
			}
		})
	})

	return
}

func processIDs(ids []string, header http.Header) (recs []string, err error) {
	var (
		// mtx sync.Mutex
		wg sync.WaitGroup
	)

	fmt.Printf("in processIDs with list %q and header %q\n", ids, header)

	for idx, id := range ids {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()

			params := url.Values{}
			params.Set("caseID", id)
			params.Set("openCaseList", "")
			params.Set("isPublic", "1")

			req, err := http.NewRequest(http.MethodPost, "https://miamidade.realtdm.com/public/cases/details", bytes.NewBufferString(params.Encode()))
			if err != nil {
				log.Fatal(err)
			}
			req.Header = header

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Fatal(err)
			}

			if resp == nil {
				log.Fatal("resp == nil")
			}
			defer resp.Body.Close()

			if resp.StatusCode >= http.StatusBadRequest {
				log.Printf("#%d: %d status code", idx, resp.StatusCode)
			}

			/****************************************************************/
			// b, _ := ioutil.ReadAll(resp.Body)
			// ioutil.WriteFile(id+".html", b, 0644)
			/****************************************************************/

			return
		}(idx, id)
	}

	wg.Wait()

	return
}

func main() {
	// h, _, err := getHeader()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	res, err := http.Get(searchSite)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	cookie := res.Header.Get("Set-Cookie")
	_ = cookie

	header := &http.Header{
		"Accept":                    []string{"text/html:application/xhtml+xml:application/xml;q=0.9:image/webp:image/apng:*/*;q=0.8"},
		"Accept-Language":           []string{"en-US:en;q=0.9:ru;q=0.8"},
		"Cache-Control":             []string{"no-cache"},
		"Content-Type":              []string{"application/x-www-form-urlencoded"},
		"Pragma":                    []string{"no-cache"},
		"Upgrade-Insecure-Requests": []string{"1"},
		"X-Compress":                []string{"null"},
	}

	params := url.Values{}
	params.Set("caseID", "21908")
	params.Set("openCaseList", "")
	params.Set("isPublic", "1")

	req, err := http.NewRequest(http.MethodPost, "https://miamidade.realtdm.com/public/cases/details", bytes.NewBufferString(params.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header = *header

	client := &http.Client{
		Timeout: time.Minute * 1,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if resp == nil {
		log.Fatal("resp == nil")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		log.Fatal("400+ status code ", resp.StatusCode)
	}

	/****************************************************************/
	// b, _ := ioutil.ReadAll(resp.Body)
	// ioutil.WriteFile("realkek.html", b, 0644)
	/****************************************************************/

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var saleDates []string
	var propertyAddresses []string
	doc.Find("div#summarySummary.toggle-container > table.table.no-borders.toggle-container-content").Each(func(i int, summary *goquery.Selection) {
		summary.Find("tr").Each(func(i int, summaryRaw *goquery.Selection) {
			rowTitle := summaryRaw.Find("td.text-right").Text()
			if rowTitle == saleDateTitle {
				// TODO: replace newline, it doesn't work properly, idk why
				saleDates = append(saleDates, strings.Replace(strings.TrimSpace(summaryRaw.Children().Eq(1).Text()), "\n", " ", -1))
			} else if rowTitle == propertyAddressTitle {
				propertyAddresses = append(propertyAddresses, strings.TrimSpace(summaryRaw.Children().Eq(1).Text()))
			}
		})
	})

	fmt.Println(saleDates)
	fmt.Printf("%#v\n", propertyAddresses)

	// result, err := getSearchResults(1, h, p)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // ioutil.WriteFile("test.html", []byte(result), 0644)

	// doc, err := goquery.NewDocumentFromReader(strings.NewReader(result))
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// checkResults := doc.Find(noResultsSelector).Text()
	// if checkResults == noCases {
	// 	fmt.Println("OK NO RESULTS")
	// 	return
	// } else if checkResults == noFilter {
	// 	fmt.Println("OK NO FILTER")
	// 	return
	// }

	// pageInfo := doc.Find(pageSelector).Text()
	// pageMatch := rxPageCount.FindStringSubmatch(pageInfo)
	// if len(pageMatch) != 3 {
	// 	log.Fatal("wrong page info")
	// }

	// pageCount, err := strconv.ParseInt(pageMatch[2], 10, 64)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// errCh := make(chan error, 1)
	// var (
	// 	mtx     sync.Mutex
	// 	wg      sync.WaitGroup
	// 	records []string
	// )

	// _ = pageCount
	// for page := 1; page <= int(1); page++ {
	// 	wg.Add(1)
	// 	go func(page int) {
	// 		defer wg.Done()

	// 		body, err := getSearchResults(page, h, p)
	// 		if err != nil {
	// 			select {
	// 			case errCh <- err:
	// 			default:
	// 			}
	// 		}

	// 		activeIDs := getActiveIDs(body)

	// 		workersTimesToExec := (len(activeIDs) / entriesToFetchPerOnce) + 1
	// 		for i := 0; i < workersTimesToExec; i++ {
	// 			var currentList []string
	// 			if len(activeIDs) > entriesToFetchPerOnce {
	// 				currentList, activeIDs = activeIDs[:entriesToFetchPerOnce], activeIDs[entriesToFetchPerOnce:]
	// 			} else {
	// 				currentList = activeIDs
	// 			}
	// 			recs, err := processIDs(currentList, *h)
	// 			if err != nil {
	// 				select {
	// 				case errCh <- err:
	// 				default:
	// 				}
	// 			} else if len(recs) > 0 {
	// 				mtx.Lock()
	// 				records = append(records, recs...)
	// 				mtx.Unlock()
	// 			}
	// 		}
	// 	}(page)
	// }
	// wg.Wait()

	// select {
	// case err = <-errCh:
	// default:
	// }

	// if err != nil {
	// 	log.Fatal(err)
	// }
	log.Println("EXIT")
}
