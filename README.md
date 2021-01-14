```
go get github.com/reneluria/time-http 
time-http -h 2>&1 > README.md
time-http
Measure time to get request
Simple make http request and returns the amount of time it took
Usage of ./time-http:
	./time-http <url1> [<url2> .. <urln>]
  -c int
    	number of requests per url (default 1)
  -i string
    	ip to send requests to
  -k	skip tls certificate verification
  -p string
    	tcp port to connect to
  -t int
    	timeout in milliseconds (default 1000)
  -w int
    	milliseconds to wait between each call (default 500)
```
