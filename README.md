# ssm-get-parameter

For a long time I was waiting for an official release of dockerfy with AWS parameter store support. I gave up. 

As 90% of my uses of dockerfy looked like:

```
dockerfy --aws-secret-prefix /mysql/root/ /bin/echo '{{.AWS_Secret.password}}'
```
I decided to write ssm-get-parameter.

A few lines of go, allows me to replace the above code with:

```
ssm-get-parameter --parameter-name  /mysql/root/password
```

