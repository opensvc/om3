__opensvc_handle_word()
{
    [ $cword -gt 1 ] && [ ! -z "${words[1]}" ] && ! __opensvc_contains_word ${words[1]} svc vol sec cfg usr ccfg nscfg all completion create daemon monitor help && {
        words[1]="all"
    }
    ___opensvc_handle_word
}       
        
___opensvc_handle_word()
{       
    if [[ $c -ge $cword ]]; then
        __opensvc_handle_reply
        return
    fi  
    __opensvc_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __opensvc_handle_flag
    elif __opensvc_contains_word "${words[c]}" "${commands[@]}"; then
        __opensvc_handle_command
    elif [[ $c -eq 0 ]]; then
        __opensvc_handle_command 
    elif __opensvc_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION}" || "${BASH_VERSINFO[0]}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __opensvc_handle_command
        else
            __opensvc_handle_noun
        fi
    else
        __opensvc_handle_noun
    fi
    __opensvc_handle_word
}
