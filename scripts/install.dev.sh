go build -o lota . && sudo cp lota /usr/bin/lota
lota --install-completion zsh
rm -f ~/.zcompdump*
exec zsh
