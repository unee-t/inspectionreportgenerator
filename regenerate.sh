#!/bin/bash -e
INPUT=$1

# aws --profile $AWS_PROFILE s3 ls --recursive s3://dev-media-unee-t/ | grep json$ | grep 2018-11-23 | while read -r _ _ _ fn; do echo s3://dev-media-unee-t/$fn; done

udomain() {
	case $1 in
		prod) echo $2.unee-t.com
		;;
		*) echo $2.$1.unee-t.com
		;;
	esac
}


if test "${INPUT##*.}" != "json"
then
	echo Require JSON file, not $INPUT
	exit 1
fi

BUCKET=$(echo $INPUT | awk -F[/:] '{print $4}')

case $BUCKET in
	prod-media-unee-t)
	STAGE=prod
	;;
	dev-media-unee-t)
	STAGE=dev
	;;
	*)
	echo Unknown bucket: $BUCKET
	exit 1
	;;
esac


AWS_PROFILE=uneet-$STAGE

# step 1 download JSON

aws --profile $AWS_PROFILE s3 cp $INPUT /tmp

# step 2 regenerate HTML file

fn=/tmp/$(basename $INPUT)
echo Working on $fn

cat <<< "$(jq ".force += true " < $fn)" > $fn

HTML=$(curl -X POST \
	https://$(udomain $STAGE pdfgen) \
	-H "Authorization: Bearer $(aws --profile $AWS_PROFILE ssm get-parameters --names API_ACCESS_TOKEN --with-decryption --query Parameters[0].Value --output text)" \
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
	https://$(udomain $STAGE prince) \
	-H 'Content-Type: application/json' \
	-H 'cache-control: no-cache' \
	-d "{ \"document_url\": \"$HTML\", \"date\": \"$date\"}"

#aws --profile uneet-dev cloudfront create-invalidation --distribution-id E2L4KVYCVKXLA1 --invalidation-batch "{ \"Paths\": { \"Quantity\": 1, \"Items\": [ \"/*\" ] }, \"CallerReference\": \"$(shell date +%s)\" }"
#aws --profile uneet-prod cloudfront create-invalidation --distribution-id E3NBG008M01XS8 --invalidation-batch "{ \"Paths\": { \"Quantity\": 1, \"Items\": [ \"/*\" ] }, \"CallerReference\": \"$(shell date +%s)\" }"
