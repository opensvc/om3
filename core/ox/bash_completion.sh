__ox_handle_word()
{
     __ox_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
     [ $c -eq 1 ] && [ $cword -gt 1 ] && {
        local k
        case "${words[c]}" in
            "") ;;
            svc|vol|sec|cfg|usr|ccfg|nscfg|ccfg|all|cluster) ;;
            pool|network|array|completion|create|daemon|mon|monitor|help) ;;
            *,*) k="all" ;;
            */svc/*|svc/*) k="svc" ;;
            */vol/*|vol/*) k="vol" ;;
            */sec/*|sec/*) k="sec" ;;
            */usr/*|usr/*) k="usr" ;;
            */cfg/*|cfg/*) k="cfg" ;;
            */nscfg|nscfg/*|*/) k="nscfg" ;;
            *=*|*\**|*\?*) k="all" ;;
            *) k="svc" ;;
        esac
        [ ! -z "$k" ] && {
          # replace "om cfg/foo" with "om cfg -s cfg/foo"
          words=("${words[@]:0:c}" "$k" "-s" "${words[c]}" "${words[@]:c+1}")
          cword=$(($cword+2))
          __ox_debug "${FUNCNAME[0]}: >>> ${words[*]}"
        }
    }
    ___ox_handle_word
}

___ox_handle_word()
{       
    if [[ $c -ge $cword ]]; then
        __ox_handle_reply
        return
    fi  
    __ox_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __ox_handle_flag
    elif __ox_contains_word "${words[c]}" "${commands[@]}"; then
        __ox_handle_command
    elif [[ $c -eq 0 ]]; then
        __ox_handle_command 
    elif __ox_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION}" || "${BASH_VERSINFO[0]}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __ox_handle_command
        else
            __ox_handle_noun
        fi
    else
        __ox_handle_noun
    fi
    __ox_handle_word
}
