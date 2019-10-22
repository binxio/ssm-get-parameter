# ssm-get-parameter
The simple utility can be used the configure the environment of an application with values from the AWS SSM Parameter store.

## How does it work?
It is simple. Specify one or more environment variables with a URI of the ssm: protocol, as follows:

```
export MYSQL_PASSWORD=ssm:///mysql/root/password
ssm-get-parameter bash -c 'echo $MYSQL_PASSWORD'
```
the utility will lookup the value of `/mysql/root/password` in the SSM parameter store and replace the environment variable.
The program on the command line will be exec'ed with MYSQL\_PASSWORD set to the actual value.

## Query parameters
The utility supports the following query parameters:

- default - value if the value could not be retrieved from the parameter store.
- destination - the filename to write the value to. value replaced with file: url.

If no default nor destination is specified and the parameter is not found, the utility will return an error.
If a default is specified and the parameter is not found, the utility will use the default.
If a destination file exists and no default is specified, the file will be read as the default value.

For example:
```
$ export ORACLE_PASSWORD=ssm:///oracle/scott/password?default=tiger&destination=/tmp/password
$ ssm-get-parameter bash -c 'echo $ORACLE_PASSWORD'
file:///tmp/password
$ cat /tmp/password
tiger
```

## Environment substitution
The URI may contain an environment variable reference. For example:
```
$ export ENV=dev
$ export 'PASSWORD=ssm:///${ENV}/mysql/root/password'
ssm-get-parameter bash -c 'echo $PASSWORD'
```
will print out the value of `/dev/mysql/root/password`.

## Dockerfile usage
To idiomatic way to use the utility is as follows:
```
FROM binxio/ssm-get-parameter

FROM alpine:3.6
COPY --from=0 /ssm-get-parameter /usr/local/bin/

ENV PGPASSWORD=ssm:///postgres/root/password
ENTRYPOINT [ "/usr/local/bin/ssm-get-parameter"]
CMD [ "/bin/bash", "-c", "echo $PGPASSWORD"]
```

## installation
If you have golang installed, type:

```
go get github.com/binxio/ssm-get-parameter
```

## installation in Docker
With Docker you can use the multi-stage build:

```
FROM binxio/ssm-get-parameter

FROM alpine:3.6
COPY --from=0 /ssm-get-parameter /usr/local/bin/
```

## download
you can download a 64 bit Linux or MacOS binary from [https://github.com/binxio/ssm-get-parameter/releases](https://github.com/binxio/ssm-get-parameter/releases).
