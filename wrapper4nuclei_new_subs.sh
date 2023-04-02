#!/bin/bash

subdomains2send=$1

filteredSubsPath="/tmp/filtered_new_subs_nuclei.$(date +%s)"

printf $subdomains2send | grep -vf /home/ec2-user/.config/cronScripts/nucleiBlacklist.txt > $filteredSubsPath

if [ -s "$filteredSubsPath" ]; then
    echo "not empty" 
    message={\"domains\":\"$(cat $filteredSubsPath)\"}
    echo $message | sed 's/ /\\n/g' > $filteredSubsPath.message
    GO111MODULE=off go run /home/ec2-user/rabbitmq/send.go -messageFile $filteredSubsPath.message -priority 99 -queue nuclei
else
  echo "$filteredSubsPath is empty after filtering blacklist"
fi

