#!/usr/bin/env bash
src="test-data"
readarray -d $'\0' dirs < <(find test-data -mindepth 1 -maxdepth 1 -type d -print0 |sort -z)

for dir in "${dirs[@]}"; do
    touch "$dir/untouched"
done

go test "$@"

for dir in "${dirs[@]}"; do
    readarray -d $'\0' testDirs < <(find "$dir" -mindepth 1 -maxdepth 1 -type d -print0 |sort -z)
    if [[ -f $dir/wantErr ]]; then
       suffixA="actualErr"
       suffixE="expectErr"
       otherA="actual"
       rm -rf "$dir/expect"
    else
        suffixA="actual"
        suffixE="expect"
        otherA="actualErr"
        rm -rf "$dir/expectErr"
    fi
    for tdir in "${testDirs[@]}"; do
        if [[ -e $tdir/$suffixA ]]; then
            diff -Ndur "$tdir/$suffixE" "$tdir/$suffixA" && continue
            read -p "Move '$tdir/$suffixA' to '$tdir/$suffixE' (y/n)" ans
            case $ans in
                y) rm -rf "$tdir/$suffixE"; mv "$tdir/$suffixA" "$tdir/$suffixE"
                   ;;
            esac
        else
            echo "Missing output at '$tdir/$suffixA'!"
            echo "Check $tdir/$otherA"
            echo
        fi
    done
done
