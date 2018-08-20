NAME=pdfgen
REPO=uneet/$(NAME)

localtest:
	curl -X POST -d @tests/test.json -H "Authorization: Bearer $$(aws --profile uneet-dev ssm get-parameters --names API_ACCESS_TOKEN --with-decryption --query Parameters[0].Value --output text)" http://localhost:3000

remotetest:
	curl -X POST -d @tests/test.json -H "Authorization: Bearer $$(aws --profile uneet-dev ssm get-parameters --names API_ACCESS_TOKEN --with-decryption --query Parameters[0].Value --output text)" https://pdfgen.dev.unee-t.com/

dev:
	@echo $$AWS_ACCESS_KEY_ID
	jq '.profile |= "uneet-dev" |.stages.staging |= (.domain = "pdfgen.dev.unee-t.com" | .zone = "dev.unee-t.com")' up.json.in > up.json
	up
