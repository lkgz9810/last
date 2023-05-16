#!/bin/bash
/usr/sbin/crond
/app/server/server 
wait -n 
exit $?
