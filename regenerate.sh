#!/bin/bash
INPUT=$1

# aws --profile uneet-dev s3 ls --recursive s3://dev-media-unee-t/ | grep json$ | grep 2018-11-23 | while read -r _ _ _ fn; do echo s3://dev-media-unee-t/$fn; done

if test "${INPUT##*.}" != "json"
then
	echo Require JSON file, not $INPUT
	exit 1
fi

# INPUT=s3://dev-media-unee-t/2018-12-10/228-b1ed678a.json
# step 1 download JSON

aws --profile uneet-dev s3 cp $INPUT /tmp

# step 2 regenerate HTML file

fn=/tmp/$(basename $INPUT)
echo Working on $fn

cat <<< "$(jq ".force += true " < $fn)" > $fn

HTML=$(curl -X POST \
  https://pdfgen.dev.unee-t.com \
  -H "Authorization: Bearer $(aws --profile uneet-dev ssm get-parameters --names API_ACCESS_TOKEN --with-decryption --query Parameters[0].Value --output text)" \
  -H 'Content-Type: application/json' \
  -H 'cache-control: no-cache' \
  --data @$fn |
  jq -r .HTML)

echo New output: $HTML

# step 3, regenerate PDF

date=$(jq -r .date < $fn)
echo Setting date: $date

# document_url must be from the host unee-t.com btw
curl -X POST \
  https://prince.dev.unee-t.com \
  -H 'Content-Type: application/json' \
  -H 'cache-control: no-cache' \
  -d "{ \"document_url\": \"$HTML\", \"date\": \"$date\"}"

#aws --profile uneet-dev cloudfront create-invalidation --distribution-id E2L4KVYCVKXLA1 --invalidation-batch "{ \"Paths\": { \"Quantity\": 1, \"Items\": [ \"/*\" ] }, \"CallerReference\": \"$(shell date +%s)\" }"
