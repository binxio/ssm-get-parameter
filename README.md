# ssm-get-parameter
The simple utility can be used in docker entrypoint scripts to obtain parameters and secrets from the AWS Parameter store. The program takes one option `--parameter-name` which is the name of parameter to get the value from. For example:

```
	ssm-get-parameter --parameter-name  /mysql/root/password
```
it writes the value of the parameter to stdout, so you can use it anyway you like. For instance,

```
MYSQL_PASSWORD=$(ssm-get-parameter --parameter-name  /mysql/root/password)
```

## installation
you can download a Linux or MacOS binary from [https://github.com/binxio/ssm-get-parameter/releases](https://github.com/binxio/ssm-get-parameter/releases).

If you have golang installed, type:

```
go get github.com/binxio/ssm-get-parameter
```

