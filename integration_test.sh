#!/bin/bash

RED=$(tput setaf 1)
GREEN=$(tput setaf 2)
RESET=$(tput sgr0)
IMAGE1="img1.jpg"
IMAGE2="img2.jpg"
IMAGE1_MD5="cf542a996e89af5e92b3e168c6610c41"
IMAGE2_MD5="eba43ef004c3446623bf2b54d97ba924"

function cache_size() {
    SIZE=$(curl -s 'http://localhost:8081/metrics' | grep ^image_previever_cache_size | cut -d " " -f 2)        
    echo $SIZE
}

function get_image50x50() {
    CODE=$(curl -s --write-out "%{http_code}\n" 'http://localhost:8081/fill/50/50/web/img/gopher.jpg' -o $IMAGE1)
    echo $CODE
}

function get_image50x100() {
    CODE=$(curl -s --write-out "%{http_code}\n" 'http://localhost:8081/fill/50/100/web/img/gopher.jpg' -o $IMAGE2)
    echo $CODE
}

function md5_image1() {
    MD5SUM=$(md5 $IMAGE1 | cut -d "=" -f 2 | sed 's/ //g')
    echo $MD5SUM
}

function md5_image2() {
    MD5SUM=$(md5 $IMAGE2 | cut -d "=" -f 2 | sed 's/ //g')
    echo $MD5SUM
}

echo "Test 1. Test the cache"

size=$(cache_size)
[ $size -eq 0 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Check the cache size is 0 ... " $STATUS

code=$(get_image50x50)
[ $code -eq 200 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Get the image1 and check the return code is 200..." $STATUS

size=$(cache_size)
[ $size -eq 1 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Check the cache size increased by 1 and result is $size ..." $STATUS

code=$(get_image50x50)
[ $code -eq 200 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Get the same image1 again and check the return code 200 ..." $STATUS

size=$(cache_size)
[ $size -eq 1 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Check the cache size is stil 1  ..." $STATUS

md5=$(md5_image1)
[ $md5 = $IMAGE1_MD5 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Check the md5 of image1 $md5 ..." $STATUS

code=$(get_image50x100)
[ $code -eq 200 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Get the image2 again and check the return code 200 ..." $STATUS

size=$(cache_size)
[ $size -eq 2 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Check the cache size is increased by 1  ..." $STATUS

md5=$(md5_image2)
[ $md5 = $IMAGE2_MD5 ] && STATUS="${GREEN}OK${RESET}" || STATUS="${RED}NOK${RESET}"
printf " %-70s %10s\n" "Check the md5 of image1 $md5 ..." $STATUS


# echo "Do image request $RET"
# SIZE=$(curl -s 'http://localhost:8081/metrics' | grep ^image_previever_cache_size | cut -d " " -f 2)
# echo "Current size is $SIZE"
