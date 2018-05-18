FROM 		golang:1.8
WORKDIR		/go
ADD		. /go/src/github.com/binxio/ssm-get-parameter
RUN		CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' github.com/binxio/ssm-get-parameter

FROM 		scratch
COPY --from=0		/go/ssm-get-parameter /
