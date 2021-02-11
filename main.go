package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"os"
	"time"
	"crypto/tls"
)

var description = `Measure time to get request
Simple make http request and returns the amount of time it took
`

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s:\n%s", os.Args[0], description)
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\t%s <url1> [<url2> .. <urln>]\n", os.Args[0])
		flag.PrintDefaults()
	}
}

type Result struct {
	URL string
	Duration time.Duration
	Err error
}

func BenchUrl(urlStr string, ch chan Result) {
	start := time.Now()

	var result = Result{URL: urlStr}
	// , Duration: time.Since(start), Err: err}

	// create a new client, no redirections
	var client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// create request
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		log.Printf("Error: cannot create http request: %v\n", err)
		result.Duration = time.Since(start)
		result.Err = err
		ch <- result
		return
	}

	// launch request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("cannot launch request: %v\n", err)
		result.Duration = time.Since(start)
		result.Err = err
		ch <- result
		return
	}
	defer client.CloseIdleConnections()

	// read body
	if _, err := ioutil.ReadAll(resp.Body); err != nil {
		log.Fatalln(err)
		result.Duration = time.Since(start)
		result.Err = err
		ch <- result
		return
	}
	defer resp.Body.Close()

	// log.Println(string(body))
	elapsed := time.Since(start)
	// fmt.Printf("%s %d %s\n", benchUrl.String(), resp.StatusCode, elapsed)
	result.Duration = elapsed
	ch <- result
}

func TimeUrls(urls []string, timeout time.Duration) ([]Result) {

	var results []Result

	// launch benches in parallel
	ch := make(chan Result)
	for _, thisUrl := range(urls) {
		go BenchUrl(thisUrl, ch)
	}

	// this will timeout
	timeoutChannel := time.After(timeout * time.Millisecond)

	// fetch all the results
	for i := 0; i < len(urls); i++ {
		select {
		case <-timeoutChannel:
			log.Println("timeout")
			return results
		case result := <-ch:
			results = append(results, result)
		}
	}
	return results
}

func main() {
	// command line arguments
	var urls []string
	var ip, port string
	var skipverify bool
	var wait int64
	timeout := flag.Int("t", 1000, "timeout in milliseconds")
	count := flag.Int("c", 1, "number of requests per url")
	flag.StringVar(&ip, "i", "", "ip to send requests to")
	flag.StringVar(&port, "p", "", "tcp port to connect to")
	flag.BoolVar(&skipverify, "k", false, "skip tls certificate verification")
	flag.Int64Var(&wait, "w", 500, "milliseconds to wait between each call")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Error: not enough arguments")
		os.Exit(1)
	}

	// check urls in arguments
	for _, arg := range(flag.Args()) {
		thisUrl, err := url.Parse(arg)
		if err != nil {
			fmt.Printf("Error: cannot parse %v as url: %v\n", arg, err)
			os.Exit(1)
		}
		if thisUrl.Scheme != "http" && thisUrl.Scheme != "https" {
			fmt.Printf("Error: unsupported url scheme in %v\n", arg)
			os.Exit(1)
		}
		urls = append(urls, thisUrl.String())
	}

	// if ip or port is specied, modify the DefaultTransport to always dial our ip
	if ip != "" || port != "" {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}
		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			colonIndex := strings.Index(addr, ":")
			if ip == "" {
				ip = addr[:colonIndex]
			}
			if port == "" {
				port = addr[colonIndex+1:]
			}
			addr = ip + ":" + port
			return dialer.DialContext(ctx, network, addr)
		}
	}

	if skipverify {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	for i := 0; i < *count; i++ {
		results := TimeUrls(urls, time.Duration(*timeout))
		for _, result := range(results) {
			fmt.Printf("%v: %v\n", result.URL, result.Duration)
		}
		// last iteration
		if i != *count - 1 {
			time.Sleep(time.Duration(wait) * time.Millisecond)
		}
	}

}

