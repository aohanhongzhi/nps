## 管理员页面

http://proxy-admin.cupb.top/login/index


## 代理

```
GOPROXY=https://goproxy.cn,direct
```

-server=jt.cupb.top:8024 -vkey=bdim8smm4o29dgoy -type=tcp

## npc打包


windows

```shell
go build -ldflags="-w -s -extldflags -static -H windowsgui" ./cmd/npc/npc.go
```
打包32位的程序
```shell
go env -w GOARCH=amd64
```

linux下

隐藏黑窗

```shell
CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -ldflags="-w -s -extldflags -static -H windowsgui" ./cmd/npc/npc.go
CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -o 白沟.exe -ldflags="-w -s -extldflags -static -H windowsgui" ./cmd/npc/npc.go
```

不隐藏黑窗

```shell
CGO_ENABLED=0 go build -ldflags="-w -s -extldflags -static -H windowsgui" ./cmd/npc/npc.go
```

```shell
CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -ldflags="-w -s -extldflags -static" ./cmd/npc/npc.go
```



> -ldflags="-H windowsgui"

32位

```shell
CGO_ENABLED=0 GOOS=windows GOARCH=386  go build -ldflags="-w -s -extldflags -static -H windowsgui" ./cmd/npc/npc.go
```

ip地址查询

https://www.ip138.com/

## nps打包

```shell
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
```

```shell
sudo nohup nps  >> /tmp/nps.log 2>&1 &
```