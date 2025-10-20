__om_handle_word()
{
    [ $cword -gt 1 ] && [ ! -z "${words[1]}" ] && ! __om_contains_word ${words[1]} svc vol sec cfg usr ccfg nscfg all completion create daemon monitor help && {
        words[1]="all"
    }
    ___om_handle_word
}       
        
___om_handle_word()
{
    if [[ $c -ge $cword ]]; then
        __om_handle_reply
        return
    fi  
    __om_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    __om_debug "${FUNCNAME[0]}: command is ${commands[*]}"
    if [[ "${words[c]}" == -* ]]; then
        __om_handle_flag
    elif __om_contains_word "${words[c]}" "${commands[@]}"; then
        __om_handle_command
    elif [[ $c -eq 0 ]]; then
        __om_handle_command
    elif __om_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION}" || "${BASH_VERSINFO[0]}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __om_handle_command
        else
            __om_handle_noun
        fi
    else
        __om_handle_noun
    fi
    __om_handle_word
}
