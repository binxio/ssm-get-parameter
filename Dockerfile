FROM 		golang:1.19
WORKDIR		/app
ADD		. /app
RUN		CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' .

FROM 		scratch
COPY --from=0		/app/ssm-get-parameter /
