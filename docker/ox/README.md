# README.md

    make version
    docker buildx build -t opensvc/ox:dev -f ./docker/ox/Dockerfile .

The image runs ox as the unprivileged user ox (uid 65532), whose client
contexts are in /home/ox/.config/opensvc:

    docker run -it --rm -v ~/.config/opensvc:/home/ox/.config/opensvc:ro opensvc/ox:dev mon
