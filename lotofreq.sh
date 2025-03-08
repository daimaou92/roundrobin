#!/usr/bin/env bash
echo "Making 10000 requests - 5 every second.."
for i in {1..10000}
do
    curl -X POST -H 'Content-Type:application/json' --data '{"a":"b"}' http://localhost:30000/json
    echo ""
    sleep 0.2
done