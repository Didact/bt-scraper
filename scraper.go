package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/Didact/my"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
)

const posturl = "http://ln.m-chan.org/getEpub.php"
const geturl = "http://ln.m-chan.org/v3/book.php?BID=%d"

var seriesexp = regexp.MustCompile(`<input type="hidden" id="series" name="series" value="(\d+)" \/>`)

var booksexp = regexp.MustCompile(`<input type="hidden" id="books" name="books" value="(.+)" \/>`)

var once sync.Once

//FLAGS
var (
	id   = flag.Int("id", 1, "specific book ID")
	from = flag.Int("from", 1, "which book ID to start from (inclusive)")
	to   = flag.Int("to", 735, "which book ID to stop at (inclusive)")
	dir  = flag.String("dir", ".", "directory to store files in (no trailing /)")
)

func main() {
	flag.Parse()
	wg := &sync.WaitGroup{}
	sm := my.NewSemaphore(100)
	for i := *from; i <= *to; i++ {
		wg.Add(1)
		sm.Request()
		go func(j int) {
			defer wg.Done()
			defer sm.Release()
			title := download(f(j))
			if title != "" {
				fmt.Println(title)
			}
		}(i)
	}
	wg.Wait()
}

func download(series, books string) string {
	if series == "" || books == "" {
		return ""
	}
	v := url.Values{}
	v.Add("series", series)
	v.Add("books", books)
	v.Add("img", "ni")
	v.Add("size", "original")
	v.Add("submit", "")
	resp, err := http.PostForm(posturl, v)
	if err != nil {
		log.Printf("POST %s values %s: %s\n", posturl, v, err)
	}
	defer resp.Body.Close()
	filename, ok := my.ParseHeaders(resp.Header, "Content-Disposition")["filename"]
	if !ok {
		filename = series + books
	}
	if len(filename) > 143 {
		filename = filename[:143]
	}
	(&once).Do(func() {
		err = os.MkdirAll(*dir, 0755)
	})
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Create(*dir + "/" + filename)
	if err != nil {
		log.Println(err)
		return ""
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Println(err)
		return ""
	}
	return filename
}

func f(id int) (series, books string) {
	resp, err := http.Get(fmt.Sprintf(geturl, id))
	if err != nil {
		log.Printf("GET id %d: %s\n", id, err)
		return "", ""
	}
	if resp.Body == nil {
		log.Printf("id %d body nil: %s\n", id, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", ""
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("readall: %s\n", err)
		return "", ""
	}
	smatch := seriesexp.FindStringSubmatch(string(body))
	bmatch := booksexp.FindStringSubmatch(string(body))
	if len(smatch) < 2 || len(bmatch) < 2 {
		log.Printf("id %d: %s\n", id, errors.New("no matches"))
		return "", ""
	}
	return smatch[1], bmatch[1]
}
