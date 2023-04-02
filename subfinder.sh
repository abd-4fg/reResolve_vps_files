#!/bin/bash

function main(){

startTime=$(date +"%a %b %d %T %Y")

timestamp=$(date +%s)

domain=$1

OutputPath="/tmp/output.$domain.subfinder.$timestamp"

logFile="/tmp/subfinder.log"

### subfinder
timeout 5m subfinder -max-time 3 -d $domain -es sonarsearch,sitedossier -all -v -o $OutputPath || echo "subfinder run error, probably timeout $OutputPath - $(hostname) $startTime" | notify -id error

## run puredns resolve here against output of subfinder; only resolved once with verified resolvers with 500 thread.
timeout 3.9h puredns resolve $OutputPath -r ~/dictionary/google.resolvers -l 500 --skip-validation -w $OutputPath.resolved  --wildcard-batch 500000  --wildcard-tests 50 || echo "puredns run error, probably timeout $OutputPath - $(hostname) $startTime" | notify -id error

echo "puredns resolve - $domain - $(date) "  | tee -a $logFile

## check if subdomain count is below threshold, if yes upload.

SubdomainsCount=$(wc -l < $OutputPath.resolved) &&

if [[ $SubdomainsCount -lt 70000 ]]
then
    cat $OutputPath.resolved | tr '[:upper:]' '[:lower:]' > $OutputPath.lowered
    echo "inserting to DB $SubdomainsCount subdomains; $OutputPath.lowered" | tee -a $logFile
    
    echo -n "$domain : $wordlist " | tee -a $logFile

    sed -i s/$/,subfinder_reResolve_vps/ $OutputPath.lowered

    GO111MODULE=off go run /home/ec2-user/dbs/insertFile.go -T subdomains -C subdomain -C2 source -filePath $OutputPath.lowered | tee -a $logFile
    
     echo "removing $OutputPath.resolved and $OutputPath and  $OutputPath.lowered"
     rm -rf $OutputPath.resolved
    rm -rf $OutputPath
    rm -rf $OutputPath.lowered
    
else
    echo "$OutputPath.resolved; subdomain count $SubdomainsCount ; not pushed $(hostname)"
    echo "$OutputPath.resolved; subdomain count $SubdomainsCount ; not pushed $(hostname) $startTime" | notify -id error
fi;

}


cat $(ls /tmp/*domainExtract | grep -vF ".immunefi.domainExtract" | grep -vF ".it.domainExtract" )  | tr '[:upper:]' '[:lower:]' |  dsieve -f 2 | sort -u  | grep -vE "^$|\*|," | grep -vFf ~/.config/cronScripts/subfinderShuffleBlacklist.txt | shuf | while read line; do
	main $line; 
done
