Install
-------

```
go get github.com/reneluria/http-timer
```

Doc
----

```
http-timer -h 2>&1 > README.md
```

Here it is
```
http-timer:
Measure time to get request
Simple make http request and returns the amount of time it took
Usage:
	http-timer <url1> [<url2> .. <urln>]
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
