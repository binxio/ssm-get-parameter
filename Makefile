include Makefile.mk

BINARIES=dist/ssm-get-parameter-$(VERSION)-linux-amd64.zip dist/ssm-get-parameter-$(VERSION)-darwin-amd64.zip
ORGANIZATION=binxio
NAME=ssm-get-parameter
GITHUB_API=https://api.github.com/repos/$(ORGANIZATION)/$(NAME)
GITHUB_UPLOAD=https://uploads.github.com/repos/$(ORGANIZATION)/$(NAME)


build: $(BINARIES)

dist/ssm-get-parameter-$(VERSION)-linux-amd64.zip: main.go
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/ssm-get-parameter main.go
	cd dist && zip ../dist/ssm-get-parameter-$(VERSION)-linux-amd64.zip ssm-get-parameter  && rm ssm-get-parameter

dist/ssm-get-parameter-$(VERSION)-darwin-amd64.zip: main.go
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -o dist/ssm-get-parameter main.go
	cd dist && zip ../dist/ssm-get-parameter-$(VERSION)-darwin-amd64.zip ssm-get-parameter  && rm ssm-get-parameter

clean:
	rm -rf dist target



.git-release-$(VERSION): $(BINARIES)
	set -e -o pipefail ; \
	shasum -a256 $(BINARIES) | sed -e 's^dist/^^' | \
	jq --raw-input --slurp \
		--arg tag $(TAG) \
		--arg release $(VERSION) \
		'{ "draft": true,  \
                   "prerelease": false,   \
                   "tag_name": $$tag,  \
                   "name": $$tag,  \
                   "body": ("release " + $$release + (split("\n") | join("\n") | ("\n```\n" + . + "```\n"))) }' | \
	curl -sS --fail \
			-d @- \
			-o .git-release-$(VERSION) \
			-H "Authorization: token $$GITHUB_API_TOKEN" \
			-H 'Content-Type: application/json' \
			-X POST \
			$(GITHUB_API)/releases

release: check-release .git-release-$(VERSION)
	for BINARY in $(BINARIES); do \
		curl --fail -sS \
			 --data-binary @$$BINARY \
			-o /dev/null \
			-X POST \
			-H "Authorization: token $$GITHUB_API_TOKEN" \
			-H 'Content-Type: application/octet-stream' \
			$(GITHUB_UPLOAD)/releases/$(shell jq -r .id .git-release-$(VERSION))/assets?name=$$(basename $${BINARY} | sed -e 's/-$(VERSION)-/-/') ; \
	done
	curl --fail -sS \
		-d '{"draft": false}'  \
		-o /dev/null \
		-X PATCH \
		-H "Authorization: token $$GITHUB_API_TOKEN" \
		-H 'Content-Type: application/json-stream' \
		$(GITHUB_API)/releases/$(shell jq -r .id .git-release-$(VERSION))
	rm .git-release-$(VERSION)
