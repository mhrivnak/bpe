#!/usr/bin/env bash

while true
do
    sleep $(echo "scale=2; 5 * $RANDOM / 32767"| bc)
    delay=$(echo "3 + $RANDOM / 3277" | bc)
    curl -s http://localhost:8080/sync?delay=$delay
done
