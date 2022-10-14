-server=jt.cupb.top:8024 -vkey=bdim8smm4o29dgoy -type=tcp


打包

```shell
CGO_ENABLED=0 go build -ldflags="-w -s -extldflags -static" ./cmd/npc/npc.go
```

linux下

```shell
CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -ldflags="-w -s -extldflags -static" ./cmd/npc/npc.go
```


ip地址查询

https://www.ip138.com/
