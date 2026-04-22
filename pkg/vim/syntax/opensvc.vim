" opensvc config file syntax
if exists("b:current_syntax")
  finish
endif

" Section headers [DEFAULT] [disk#0] etc.
syn match opensvcSection        /^\[.\{-}\]/

" Key scope @<scope> within a key
syn match opensvcScope          /@\w\+/ contained

" Keys (word before the =), optionally with @scope
syn match opensvcKey            /^\s*\w\+\(@\w\+\)\?\ze\s*=/ contains=opensvcScope

" Equal sign
syn match opensvcEqual          /=/

" Values in curly braces {name} {fqdn} etc.
syn match opensvcReference      /{[^}]*}/

" Comments
syn match opensvcComment        /^\s*[#;].*/

hi def opensvcSection           ctermfg=Yellow      cterm=bold      guifg=#FFD700   gui=bold
hi def opensvcKey               ctermfg=Cyan                        guifg=#00BFFF
hi def opensvcScope             ctermfg=Magenta     cterm=bold      guifg=#FF00FF   gui=bold
hi def opensvcEqual             ctermfg=DarkGray                    guifg=#666666
hi def opensvcReference         ctermfg=Green       cterm=bold      guifg=#00FF7F   gui=bold
hi def opensvcComment           ctermfg=DarkGray    cterm=italic    guifg=#555555   gui=italic

let b:current_syntax = "opensvc"
