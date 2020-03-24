#!/bin/bash

export AWS_PROFILE=integration-test
export AWS_REGION=eu-central-1

function generate_password {
	head /dev/urandom | LC_ALL=C tr -dc A-Za-z0-9 | head -c 20
}

function ssm_get_parameter {
	go run main.go "$@"
}
function assert_equals {
	if [[ $1 == $2 ]] ; then
		echo "INFO: ${FUNCNAME[1]} ok" >&2
	else
		echo "ERROR: ${FUNCNAME[1]} expected '$1'  got '$2'" >&2
	fi
}

function test_simple_get {
	local result expect
	expect=$(generate_password)
	aws ssm put-parameter --name /mysql/root/password --value "$expect" --type SecureString --overwrite  > /dev/null
	result=$(ssm_get_parameter --name /mysql/root/password)
	assert_equals $expect $result
}

function test_get_via_env {
	local result expect
	expect=$(generate_password)
	aws ssm put-parameter --name /mysql/root/password --value "$expect" --type SecureString --overwrite  > /dev/null
	result=$(MYSQL_PASSWORD=ssm:///mysql/root/password ssm_get_parameter bash -c 'echo $MYSQL_PASSWORD')
	assert_equals $expect $result
}

function test_get_via_env_default {
	local result expect
	expect=$(generate_password)
	result=$(MYSQL_PASSWORD="ssm:///there-is-no-such-parameter-in-the-store-is-there?default=$expect" ssm_get_parameter bash -c 'echo $MYSQL_PASSWORD')
	assert_equals $expect $result
}

function test_template_format {
	local result expect password
	password=$(generate_password)
	aws ssm put-parameter --name /postgres/kong/password --value "$password" --type SecureString --overwrite  > /dev/null
	expect="localhost:5432:kong:kong:${password}"

	result=$(TMP=/tmp \
	         PGPASSFILE=ssm:///postgres/kong/password?template='localhost:5432:kong:kong:{{.}}%0A&destination=$TMP/.pgpass' \
		ssm_get_parameter bash -c 'cat $PGPASSFILE')
	assert_equals $expect $result
}

function test_env_substitution {
	local result expect
	expect=$(generate_password)
	aws ssm put-parameter --name /$expect/mysql/root/password --value "$expect" --type SecureString --overwrite  > /dev/null
	result=$(ENV=$expect \
                PASSWORD='ssm:///${ENV}/mysql/root/password' \
	        ssm_get_parameter bash -c 'echo $PASSWORD')
	assert_equals $expect $result
}

function test_destination {
	local result expect filename
	expect=$(generate_password)
	filename=/tmp/password-$$
	aws ssm put-parameter --name /postgres/kong/password --value "$expect" --type SecureString --overwrite  > /dev/null

	result=$(FILENAME=$filename \
	         PASSWORD_FILE='ssm:///postgres/kong/password?destination=$FILENAME&chmod=0600' \
		ssm_get_parameter bash -c 'echo $PASSWORD_FILE')
	assert_equals $filename $result
	assert_equals $expect $(<$filename)
	assert_equals 600 $(stat -f %A $filename)
	rm $filename
}

function test_destination_default {
	local result expect filename
	expect=$(generate_password)
	filename=/tmp/password-$$
	echo -n "$expect" > $filename
	result=$(FILENAME=$filename \
	         PASSWORD_FILE='ssm:///there-is-no-such-parameter-in-the-store-is-there?destination=$FILENAME&chmod=0600' \
		ssm_get_parameter bash -c 'echo $PASSWORD_FILE')
	assert_equals $filename $result
	assert_equals $expect $(<$filename)
	assert_equals 600 $(stat -f %A $filename)
	rm $filename
}

function main {
	test_simple_get
	test_get_via_env
	test_get_via_env_default
	test_destination
	test_destination_default
	test_env_substitution
	test_template_format
}

main
