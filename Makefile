NAME=pdfgen
REPO=uneet/$(NAME)

dev:
	@echo $$AWS_ACCESS_KEY_ID
	jq '.profile |= "uneet-dev" |.stages.staging |= (.domain = "pdfgen.dev.unee-t.com" | .zone = "dev.unee-t.com")' up.json.in > up.json
	up
