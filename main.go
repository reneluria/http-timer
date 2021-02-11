package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
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
	URL      string
	Duration time.Duration
	Err      error
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

func TimeUrls(urls []string, timeout time.Duration) []Result {

	var results []Result

	// launch benches in parallel
	ch := make(chan Result)
	for _, thisUrl := range urls {
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

func minSlice(values []time.Duration) time.Duration {
	var min time.Duration
	for i, val := range values {
		if i == 0 || val < min {
			min = val
		}
	}
	return min.Truncate(time.Millisecond)
}

func maxSlice(values []time.Duration) time.Duration {
	var max time.Duration
	for i, val := range values {
		if i == 0 || val > max {
			max = val
		}
	}
	return max.Truncate(time.Millisecond)
}

func avgSlice(values []time.Duration) time.Duration {
	var total time.Duration
	for _, val := range values {
		total += val
	}
	return time.Duration(float64(total) / float64(len(values))).Truncate(time.Millisecond)
}

func main() {
	// command line arguments
	var urls []string
	var ip, port string
	var skipverify, quiet bool
	var wait, reportTimer int64
	var count int

	timeout := flag.Int("t", 1000, "timeout in milliseconds")
	flag.IntVar(&count, "c", 1, "number of requests per url")
	flag.StringVar(&ip, "i", "", "ip to send requests to")
	flag.StringVar(&port, "p", "", "tcp port to connect to")
	flag.BoolVar(&skipverify, "k", false, "skip tls certificate verification")
	flag.Int64Var(&wait, "w", 500, "milliseconds to wait between each call")
	flag.BoolVar(&quiet, "quiet", false, "dont show that much output")
	flag.Int64Var(&reportTimer, "report-interval", 5, "report timings at this interval")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Error: not enough arguments")
		os.Exit(1)
	}

	// check urls in arguments
	for _, arg := range flag.Args() {
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

	var countOK, countTimeout int

	ticker := time.NewTicker(time.Second * time.Duration(reportTimer))
	defer ticker.Stop()

	// a slice to contain results
	var timings []time.Duration

	for i := 0; i < count; i++ {
		results := TimeUrls(urls, time.Duration(*timeout))
		var duration time.Duration
		for _, result := range results {
			if !quiet {
				fmt.Printf("%v: %v\n", result.URL, result.Duration)
			}
			duration += result.Duration
		}
		if len(results) < len(urls) {
			countTimeout++
		} else {
			countOK++
			timings = append(timings, time.Duration(float64(duration)/float64(len(results))))
		}

		// display every now and then
		select {
		case <-ticker.C:
			log.Printf("%d/%d ok, %d timeout (%.02f%%) %v/%v/%v\n",
				countOK, i, countTimeout,
				float64(countTimeout*100)/float64(i),
				minSlice(timings), avgSlice(timings), maxSlice(timings))
		default:
		}
		// last iteration
		if i != count-1 {
			time.Sleep(time.Duration(wait) * time.Millisecond)
		}
	}

	fmt.Println("Summary:")
	fmt.Printf("%d/%d ok, %d timeout (%.02f%%) %v/%v/%v\n",
		countOK, countOK+countTimeout, countTimeout,
		float64(countTimeout*100)/float64(countOK+countTimeout),
		minSlice(timings), avgSlice(timings), maxSlice(timings))

}
