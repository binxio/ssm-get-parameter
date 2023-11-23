FROM 		golang:1.21
WORKDIR		/app
ADD		. /app
RUN		CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' .

FROM 		scratch
COPY --from=0       /etc/passwd /etc/group /etc/
COPY --from=0       /root /root
COPY --from=0       /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=0		/app/ssm-get-parameter /
ENTRYPOINT ["/ssm-get-parameter"]