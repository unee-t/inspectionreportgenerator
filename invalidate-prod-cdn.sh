#!/bin/bash -x
echo Invalidating $1
aws --profile uneet-prod cloudfront create-invalidation --distribution-id E3NBG008M01XS8 \
	--invalidation-batch "{ \"Paths\": { \"Quantity\": 1, \"Items\": [ \"$1/*\" ] }, \"CallerReference\": \"$(date +%s)\" }"
