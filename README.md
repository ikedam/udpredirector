How to compile:

```
GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w" -o udpredirector main.go
```