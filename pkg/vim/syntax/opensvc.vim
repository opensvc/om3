if exists("b:current_syntax")
  finish
endif

" Section headers [DEFAULT] [disk#0] etc.
syn match opensvcSection        /^\[.\{-}\]/

" Keys (word before the =)
syn match opensvcKey            /^\s*\w\+\ze\s*=/

" Equal sign
syn match opensvcEqual          /=/

" Values in curly braces {name} {fqdn} etc.
syn match opensvcTemplate       /{[^}]*}/

" Comments
syn match opensvcComment        /^\s*[#;].*/

hi def opensvcSection           ctermfg=Yellow      cterm=bold      guifg=#FFD700   gui=bold
hi def opensvcKey               ctermfg=Cyan                        guifg=#00BFFF
hi def opensvcEqual             ctermfg=DarkGray                    guifg=#666666
hi def opensvcTemplate          ctermfg=Green       cterm=bold      guifg=#00FF7F   gui=bold
hi def opensvcComment           ctermfg=DarkGray    cterm=italic    guifg=#555555   gui=italic

let b:current_syntax = "opensvc"
